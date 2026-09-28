package content

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var defaultVault *ContentVault

// ContentVault is the top-level service that wires together the upload
// pipeline, in-memory catalog, access manager, optional SQLite persistence
// and an in-memory blob store for encrypted chunk payloads. It is the
// dependency that all HTTP handlers in this file operate on.
//
// The vault is concurrency-safe. The in-memory blob store maps contentID ->
// (manifest, encryptedChunkDataByIndex). In a production deployment the blob
// store would be replaced by distributed storage nodes, but the HTTP contract
// remains identical.
type ContentVault struct {
	mu       sync.RWMutex
	pipeline *UploadPipeline
	catalog  *Catalog
	access   *AccessManager
	db       *sql.DB
	blobs    map[string]*blobEntry
}

type blobEntry struct {
	manifest   *ContentManifest
	chunks     map[int][]byte // index -> encrypted chunk data
	original   []byte         // plaintext (kept for streaming convenience)
	uploadedAt time.Time
}

// NewContentVault constructs a vault backed by the given pipeline. If db is
// non-nil, manifests and catalog entries are also persisted. Access policies
// are kept in-memory via the AccessManager.
func NewContentVault(pipeline *UploadPipeline, db *sql.DB) *ContentVault {
	v := &ContentVault{
		pipeline: pipeline,
		catalog:  NewCatalog(),
		access:   NewAccessManager(),
		db:       db,
		blobs:    make(map[string]*blobEntry),
	}
	if db != nil {
		if err := EnsureContentTables(db); err != nil {
			log.Printf("Error EnsureContentTables: %v", err)
		}
	}
	return v
}

// Catalog exposes the in-memory catalog (used by tests and admin code).
func (v *ContentVault) Catalog() *Catalog { return v.catalog }

// Access exposes the access manager.
func (v *ContentVault) Access() *AccessManager { return v.access }

// Pipeline exposes the upload pipeline.
func (v *ContentVault) Pipeline() *UploadPipeline { return v.pipeline }

// DB exposes the underlying database (may be nil).
func (v *ContentVault) DB() *sql.DB { return v.db }

// --------------------------- HTTP helpers ---------------------------

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Best-effort; headers already sent.
		_, _ = io.WriteString(w, `{"error":"encode failed"}`)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// extractContentID pulls the {id} path parameter from a request whose path
// looks like /api/content/{id} or /api/content/{id}/stream. It returns the
// content id and an empty suffix (the caller disambiguates handlers).
func extractContentID(path string) (id string, ok bool) {
	const prefix = "/api/content/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, "/")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return "", false
	}
	// id is everything up to the next slash.
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		rest = rest[:idx]
	}
	return rest, rest != ""
}

// userIDFromRequest extracts the caller identity from query/form or headers.
// It checks, in order: the "X-User-Id" header, the "user_id" query param, and
// the "user_id" form field. Returns "" when anonymous.
func userIDFromRequest(r *http.Request) string {
	if uid, ok := r.Context().Value("userID").(string); ok && uid != "" {
		return uid
	}
	return ""
}

// isPremiumFromRequest reports whether the caller is a premium subscriber.
// Only JWT claims in context count — headers and query must not raise access.
func isPremiumFromRequest(r *http.Request) bool {
	if v, ok := r.Context().Value("premium").(bool); ok {
		return v
	}
	return false
}

func denyContentAccess(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	writeError(w, http.StatusForbidden, "access denied")
	return true
}

// --------------------------- Upload ---------------------------

// uploadResponse is returned on successful upload.
type uploadResponse struct {
	ID          string `json:"id"`
	ManifestID  string `json:"manifestId"`
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	TotalChunks int    `json:"totalChunks"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Access      string `json:"access"`
}

// HandleContentUpload accepts a multipart/form-data POST with fields:
//   - file:        the file payload (required)
//   - title:       display title (optional, defaults to filename)
//   - type:        content type: video|audio|document|image (optional)
//   - access:      public|private|paid (default public)
//   - uploader:    uploader id (default from X-User-Id)
//   - allowed_users: comma-separated user ids for private/paid (optional)
//
// The file is run through the UploadPipeline (chunk + encrypt + manifest), the
// resulting encrypted chunks are stored in the in-memory blob store, and the
// manifest + catalog entry are persisted if a DB is configured.
func (v *ContentVault) HandleContentUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit body size to 256 MiB to bound memory.
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)

	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = header.Filename
	}
	ctype := strings.TrimSpace(r.FormValue("type"))
	if ctype == "" {
		ctype = string(ContentDocument)
	}
	accessLevel := strings.TrimSpace(r.FormValue("access"))
	if accessLevel == "" {
		accessLevel = string(AccessPublic)
	}
	uploader := userIDFromRequest(r)
	if uploader == "" {
		uploader = strings.TrimSpace(r.FormValue("uploader"))
	}
	var allowedUsers []string
	if au := strings.TrimSpace(r.FormValue("allowed_users")); au != "" {
		for _, u := range strings.Split(au, ",") {
			if u = strings.TrimSpace(u); u != "" {
				allowedUsers = append(allowedUsers, u)
			}
		}
	}

	// Track progress.
	pr := NewProgressReporter(ProgressUpload, header.Size)
	pr.SetTotal(header.Size)

	// Read the full file into a buffer, accounting for progress as we go.
	// We buffer so that we can both feed the pipeline and re-derive the
	// encrypted chunk map for the blob store (ProcessUpload does not expose
	// the encrypted chunks it produces internally).
	storedOriginal, err := readAllTracked(file, pr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read upload: "+err.Error())
		return
	}

	// Run the pipeline. We reuse the shared pipeline instance so that the
	// MasterKey is consistent for later download/decrypt.
	manifest, err := v.pipeline.ProcessUpload(header.Filename, strings.NewReader(string(storedOriginal)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload pipeline failed: "+err.Error())
		return
	}
	pr.Complete()

	// Build the encrypted chunk map for the blob store by re-chunking.
	chunkMap := make(map[int][]byte)
	if len(storedOriginal) > 0 {
		chunks, cerr := ChunkFile(strings.NewReader(string(storedOriginal)), v.pipeline.ChunkSize)
		if cerr == nil {
			for _, c := range chunks {
				ec, eerr := EncryptChunk(v.pipeline.MasterKey, c)
				if eerr == nil {
					chunkMap[c.Index] = ec.Data
				}
			}
		}
	}

	contentID := manifest.ID
	v.mu.Lock()
	v.blobs[contentID] = &blobEntry{
		manifest:   manifest,
		chunks:     chunkMap,
		original:   storedOriginal,
		uploadedAt: time.Now(),
	}
	v.mu.Unlock()

	// Catalog entry.
	entry := ContentEntry{
		ID:         contentID,
		Title:      title,
		Type:       ContentType(ctype),
		Size:       manifest.FileSize,
		Uploader:   uploader,
		ManifestID: manifest.ID,
		Access:     accessLevel,
	}
	if err := v.catalog.Add(entry); err != nil {
		log.Printf("Error catalog Add: %v", err)
	}

	// Access policy.
	v.access.SetPolicy(AccessPolicy{
		ContentID:    contentID,
		Level:        AccessLevel(accessLevel),
		AllowedUsers: allowedUsers,
	})

	// Persistence (best-effort).
	if v.db != nil {
		sm := manifestToStored(manifest, storedOriginal)
		if err := SaveManifest(v.db, sm); err != nil {
			log.Printf("Error SaveManifest: %v", err)
		}
		ne := entry
		if err := SaveCatalogEntry(v.db, &ne); err != nil {
			log.Printf("Error SaveCatalogEntry: %v", err)
		}
	}

	writeJSON(w, http.StatusCreated, uploadResponse{
		ID:          contentID,
		ManifestID:  manifest.ID,
		FileName:    manifest.FileName,
		FileSize:    manifest.FileSize,
		TotalChunks: manifest.TotalChunks,
		Title:       title,
		Type:        ctype,
		Access:      accessLevel,
	})
}

// readAllTracked reads all bytes from r, invoking reporter.Record for each
// chunk read. It returns the full buffer.
func readAllTracked(r io.Reader, reporter *ProgressReporter) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 32*1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			reporter.Record(n)
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
}

func manifestToStored(m *ContentManifest, payload []byte) *StoredManifest {
	return &StoredManifest{
		ID:                 m.ID,
		FileName:           m.FileName,
		FileSize:           m.FileSize,
		ChunkSize:          m.ChunkSize,
		TotalChunks:        m.TotalChunks,
		ErasureK:           m.ErasureK,
		ErasureN:           m.ErasureN,
		EncryptedMasterKey: m.EncryptedMasterKey,
		Chunks:             m.Chunks,
		CreatedAt:          m.CreatedAt.Unix(),
		Payload:            payload,
	}
}

// --------------------------- Download ---------------------------

// HandleContentDownload serves GET /api/content/{id}. It retrieves the stored
// encrypted chunks, decrypts and assembles them via the upload pipeline, and
// streams the original bytes back. Access is enforced through the AccessManager.
func (v *ContentVault) HandleContentDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, ok := extractContentID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing content id")
		return
	}

	v.mu.RLock()
	entry, exists := v.blobs[id]
	v.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "content not found")
		return
	}

	if denyContentAccess(w, v.access.CheckAccess(id, userIDFromRequest(r), isPremiumFromRequest(r))) {
		return
	}

	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.FormatInt(entry.manifest.FileSize, 10))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Id", id)
		w.WriteHeader(http.StatusOK)
		return
	}

	pr := NewProgressReporter(ProgressDownload, entry.manifest.FileSize)
	data, err := v.pipeline.ProcessDownload(entry.manifest, entry.chunks)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "download failed: "+err.Error())
		return
	}
	pr.Record(len(data))
	pr.Complete()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+entry.manifest.FileName+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.Header().Set("X-Content-Id", id)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// --------------------------- Stream ---------------------------

// HandleContentStream serves GET /api/content/{id}/stream. It returns an HLS
// (m3u8) playlist built from the stored content. Access is enforced.
func (v *ContentVault) HandleContentStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, ok := extractContentID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing content id")
		return
	}

	v.mu.RLock()
	entry, exists := v.blobs[id]
	v.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "content not found")
		return
	}

	if denyContentAccess(w, v.access.CheckAccess(id, userIDFromRequest(r), isPremiumFromRequest(r))) {
		return
	}

	// Build HLS segments from the original (decrypted) bytes.
	data := entry.original
	if data == nil {
		// Fallback: decrypt on the fly.
		decrypted, err := v.pipeline.ProcessDownload(entry.manifest, entry.chunks)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "stream decrypt failed: "+err.Error())
			return
		}
		data = decrypted
	}

	session := NewStreamSession(id, data, 10*time.Second)
	playlist := session.GeneratePlaylist()

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, playlist)
}

// --------------------------- List ---------------------------

// listResponse wraps a paginated catalog listing.
type listResponse struct {
	Items  []ContentEntry `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// HandleContentList serves GET /api/content with optional query filters and
// pagination: ?type=&uploader=&access=&limit=&offset=.
