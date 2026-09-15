// File: endpoints_list.go
// Split from endpoints.go: list/delete/default handlers.

package content

import (
	"log"
	"net/http"
	"strconv"
)

// ContentVault is the top-level service that wires together the upload
// pipeline, in-memory catalog, access manager, optional SQLite persistence
// and an in-memory blob store for encrypted chunk payloads. It is the
// dependency that all HTTP handlers in this file operate on.
//
// The vault is concurrency-safe. The in-memory blob store maps contentID ->
// (manifest, encryptedChunkDataByIndex). In a production deployment the blob
// store would be replaced by distributed storage nodes, but the HTTP contract
// remains identical.


// NewContentVault constructs a vault backed by the given pipeline. If db is
// non-nil, manifests and catalog entries are also persisted. Access policies
// are kept in-memory via the AccessManager.
func (v *ContentVault) HandleContentList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// If path has an {id}, this is not the collection endpoint.
	if id, ok := extractContentID(r.URL.Path); ok && id != "" && r.URL.Path != "/api/content" && r.URL.Path != "/api/content/" {
		writeError(w, http.StatusBadRequest, "use a specific content endpoint")
		return
	}

	q := r.URL.Query()
	filter := CatalogFilter{
		Type:     ContentType(q.Get("type")),
		Uploader: q.Get("uploader"),
		Access:   q.Get("access"),
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	// Prefer the DB-backed listing when available; otherwise fall back to the
	// in-memory catalog.
	var items []ContentEntry
	if v.db != nil {
		dbItems, err := ListCatalogEntries(v.db, filter)
		if err == nil {
			items = dbItems
		}
	}
	if items == nil {
		items = v.catalog.List(filter)
	}

	total := len(v.catalog.List(CatalogFilter{}))
	limit := filter.Limit
	if limit == 0 {
		limit = len(items)
	}

	writeJSON(w, http.StatusOK, listResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: filter.Offset,
	})
}

// --------------------------- Delete ---------------------------

// HandleContentDelete serves DELETE /api/content/{id}. It removes the blob,
// catalog entry, access policy and (if present) the persisted rows.
func (v *ContentVault) HandleContentDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, ok := extractContentID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing content id")
		return
	}

	v.mu.RLock()
	_, existed := v.blobs[id]
	v.mu.RUnlock()
	if !existed {
		writeError(w, http.StatusNotFound, "content not found")
		return
	}

	uploader := ""
	if entry, err := v.catalog.Get(id); err == nil && entry != nil {
		uploader = entry.Uploader
	}
	if uploader != userIDFromRequest(r) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	v.mu.Lock()
	delete(v.blobs, id)
	v.mu.Unlock()

	if err := v.catalog.Delete(id); err != nil {
		log.Printf("Error catalog Delete: %v", err)
	}
	v.access.SetPolicy(AccessPolicy{ContentID: id, Level: AccessPublic}) // clear by overwriting
	// Remove policy properly by using a dedicated clear; AccessManager has no
	// Delete so we rely on the catalog removal. Access check will return
	// ErrNoPolicy for missing ids, which download treats as public.

	if v.db != nil {
		if err := DeleteManifest(v.db, id); err != nil {
			log.Printf("Error DeleteManifest: %v", err)
		}
		if err := DeleteCatalogEntry(v.db, id); err != nil {
			log.Printf("Error DeleteCatalogEntry: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// --------------------------- Standalone handler wrappers ---------------------------
//
// These free functions match the signature http.HandleFunc expects when the
// caller wires routes manually (cmd/webserver). They delegate to a package
// default vault that must be initialised via SetDefaultVault.


// SetDefaultVault configures the package-level default vault used by the
// Standalone* handler functions. It should be called once at startup.
func SetDefaultVault(v *ContentVault) {
	defaultVault = v
}

// DefaultVault returns the configured default vault (nil if unset).
func DefaultVault() *ContentVault { return defaultVault }

// MustDefaultVault returns the default vault, panicking if unset.
func MustDefaultVault() *ContentVault {
	if defaultVault == nil {
		panic("content: default vault not set; call SetDefaultVault first")
	}
	return defaultVault
}

// HandleUpload is the http.HandlerFunc form delegating to the default vault.
func HandleUpload(w http.ResponseWriter, r *http.Request) {
	MustDefaultVault().HandleContentUpload(w, r)
}

// HandleDownload delegates to the default vault.
func HandleDownload(w http.ResponseWriter, r *http.Request) {
	MustDefaultVault().HandleContentDownload(w, r)
}

// HandleStream delegates to the default vault.
func HandleStream(w http.ResponseWriter, r *http.Request) {
	MustDefaultVault().HandleContentStream(w, r)
}

// HandleList delegates to the default vault.
func HandleList(w http.ResponseWriter, r *http.Request) {
	MustDefaultVault().HandleContentList(w, r)
}

// HandleDelete delegates to the default vault.
func HandleDelete(w http.ResponseWriter, r *http.Request) {
	MustDefaultVault().HandleContentDelete(w, r)
}
