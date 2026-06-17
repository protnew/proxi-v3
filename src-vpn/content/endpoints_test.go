package content

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// newTestVault builds a ContentVault with a small-chunk pipeline suitable for
// tests, backed by an in-memory SQLite DB.
func newTestVault(t *testing.T) *ContentVault {
	t.Helper()
	p := NewUploadPipeline(256, 2, 4) // 256-byte chunks
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	p.MasterKey = key
	db := newTestDB(t)
	return NewContentVault(p, db)
}

// uploadViaHTTP performs a multipart upload through the vault handler and
// returns the parsed uploadResponse.
func uploadViaHTTP(t *testing.T, v *ContentVault, filename string, data []byte, fields map[string]string) uploadResponse {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, val := range fields {
		if err := writer.WriteField(k, val); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/content", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// Propagate fields that callers may have put on the request via headers.
	if uid, ok := fields["_header_user_id"]; ok {
		req.Header.Set("X-User-Id", uid)
	}
	if prem, ok := fields["_header_premium"]; ok {
		req.Header.Set("X-Premium", prem)
	}

	rec := httptest.NewRecorder()
	v.HandleContentUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal upload response: %v; body=%s", err, rec.Body.String())
	}
	return resp
}

// downloadViaHTTP performs a GET /api/content/{id} and returns the body bytes.
func downloadViaHTTP(t *testing.T, v *ContentVault, id string, headers map[string]string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/content/"+id, nil)
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	rec := httptest.NewRecorder()
	v.HandleContentDownload(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func TestEndpoint_UploadDownloadRoundtrip(t *testing.T) {
	v := newTestVault(t)

	original := bytes.Repeat([]byte("ProxiVault-"), 200) // ~2200 bytes → 9 chunks

	resp := uploadViaHTTP(t, v, "roundtrip.bin", original, map[string]string{
		"title":    "Roundtrip Test",
		"type":     "document",
		"access":   "public",
		"uploader": "tester",
	})

	if resp.FileName != "roundtrip.bin" {
		t.Errorf("FileName = %q, want %q", resp.FileName, "roundtrip.bin")
	}
	if resp.FileSize != int64(len(original)) {
		t.Errorf("FileSize = %d, want %d", resp.FileSize, len(original))
	}
	if resp.TotalChunks == 0 {
		t.Error("expected >0 chunks")
	}
	if resp.ID == "" {
		t.Fatal("empty content ID")
	}

	code, downloaded := downloadViaHTTP(t, v, resp.ID, nil)
	if code != http.StatusOK {
		t.Fatalf("download status = %d, want %d; body=%s", code, http.StatusOK, string(downloaded))
	}
	if !bytes.Equal(original, downloaded) {
		t.Fatalf("download mismatch: got %d bytes, want %d bytes", len(downloaded), len(original))
	}
}

func TestEndpoint_UploadDownloadEmptyFile(t *testing.T) {
	v := newTestVault(t)

	resp := uploadViaHTTP(t, v, "empty.bin", []byte{}, map[string]string{
		"title":  "Empty",
		"type":   "document",
		"access": "public",
	})
	if resp.TotalChunks != 0 {
		t.Errorf("TotalChunks = %d, want 0", resp.TotalChunks)
	}

	code, downloaded := downloadViaHTTP(t, v, resp.ID, nil)
	if code != http.StatusOK {
		t.Fatalf("download empty status = %d", code)
	}
	if len(downloaded) != 0 {
		t.Errorf("downloaded %d bytes, want 0", len(downloaded))
	}
}

func TestEndpoint_DownloadMissing(t *testing.T) {
	v := newTestVault(t)
	code, body := downloadViaHTTP(t, v, "nonexistent-id", nil)
	if code != http.StatusNotFound {
		t.Errorf("download missing status = %d, want %d; body=%s", code, http.StatusNotFound, string(body))
	}
}

func TestEndpoint_UploadWrongMethod(t *testing.T) {
	v := newTestVault(t)
	req := httptest.NewRequest(http.MethodGet, "/api/content", nil)
	rec := httptest.NewRecorder()
	v.HandleContentUpload(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET upload status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestEndpoint_UploadMissingFile(t *testing.T) {
	v := newTestVault(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("title", "no file")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/content", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	v.HandleContentUpload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("upload without file status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEndpoint_StreamReturnsValidM3U8(t *testing.T) {
	v := newTestVault(t)

	// Use a payload large enough to produce at least 2 HLS segments.
	segBytes := BytesPerSecond * 10 // 10s segment
	data := bytes.Repeat([]byte("STREAMDATA"), segBytes/10+5)

	resp := uploadViaHTTP(t, v, "video.mp4", data, map[string]string{
		"title":  "Stream Video",
		"type":   "video",
		"access": "public",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/content/"+resp.ID+"/stream", nil)
	rec := httptest.NewRecorder()
	v.HandleContentStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "mpegurl") {
		t.Errorf("Content-Type = %q, want mpegurl variant", ct)
	}

	playlist := rec.Body.String()
	required := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		"#EXT-X-TARGETDURATION:10",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXTINF:",
		".ts",
		"#EXT-X-ENDLIST",
	}
	for _, want := range required {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q\nfull playlist:\n%s", want, playlist)
		}
	}
}

func TestEndpoint_StreamMissingContent(t *testing.T) {
	v := newTestVault(t)
	req := httptest.NewRequest(http.MethodGet, "/api/content/nope/stream", nil)
	rec := httptest.NewRecorder()
	v.HandleContentStream(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("stream missing status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestEndpoint_ListWithPagination(t *testing.T) {
	v := newTestVault(t)

	// Upload 5 items with distinct content (content-addressed IDs dedup
	// identical payloads, so each must differ).
	for i := 0; i < 5; i++ {
		payload := bytes.Repeat([]byte{byte('A' + i)}, 100+i)
		uploadViaHTTP(t, v, "file"+string(rune('0'+i))+".bin", payload, map[string]string{
			"title":    "Item " + string(rune('0'+i)),
			"type":     "document",
			"access":   "public",
			"uploader": "alice",
		})
	}

	// List all.
	listReq := httptest.NewRequest(http.MethodGet, "/api/content", nil)
	listRec := httptest.NewRecorder()
	v.HandleContentList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRec.Code)
	}
	var lr listResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(lr.Items) != 5 {
		t.Errorf("list all count = %d, want 5", len(lr.Items))
	}
	if lr.Total != 5 {
		t.Errorf("Total = %d, want 5", lr.Total)
	}

	// List with limit=2, offset=0.
	listReq = httptest.NewRequest(http.MethodGet, "/api/content?limit=2&offset=0", nil)
	listRec = httptest.NewRecorder()
	v.HandleContentList(listRec, listReq)
	if err := json.Unmarshal(listRec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("unmarshal list page1: %v", err)
	}
	if len(lr.Items) != 2 {
		t.Errorf("page1 count = %d, want 2", len(lr.Items))
	}
	if lr.Offset != 0 {
		t.Errorf("page1 Offset = %d, want 0", lr.Offset)
	}

	// List with limit=2, offset=2.
	listReq = httptest.NewRequest(http.MethodGet, "/api/content?limit=2&offset=2", nil)
	listRec = httptest.NewRecorder()
	v.HandleContentList(listRec, listReq)
	if err := json.Unmarshal(listRec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("unmarshal list page2: %v", err)
	}
	if len(lr.Items) != 2 {
		t.Errorf("page2 count = %d, want 2", len(lr.Items))
	}

	// Filter by access.
	listReq = httptest.NewRequest(http.MethodGet, "/api/content?access=public", nil)
	listRec = httptest.NewRecorder()
	v.HandleContentList(listRec, listReq)
	if err := json.Unmarshal(listRec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("unmarshal list public: %v", err)
	}
	if len(lr.Items) != 5 {
		t.Errorf("public count = %d, want 5", len(lr.Items))
	}
}

func TestEndpoint_AccessControlPublic(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("pub"), 50)
	resp := uploadViaHTTP(t, v, "pub.bin", data, map[string]string{
		"title":  "Public",
		"type":   "document",
		"access": "public",
	})

	// Anonymous user can download public content.
	code, body := downloadViaHTTP(t, v, resp.ID, nil)
	if code != http.StatusOK {
		t.Errorf("public download status = %d, want 200; body=%s", code, string(body))
	}
}

func TestEndpoint_AccessControlPrivate(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("priv"), 50)
	resp := uploadViaHTTP(t, v, "priv.bin", data, map[string]string{
		"title":         "Private",
		"type":          "document",
		"access":        "private",
		"uploader":      "owner",
		"allowed_users": "alice,bob",
	})

	// Allowed user alice → 200.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{"X-User-Id": "alice"}); code != http.StatusOK {
		t.Errorf("alice download status = %d, want 200", code)
	}
	// Allowed user bob → 200.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{"X-User-Id": "bob"}); code != http.StatusOK {
		t.Errorf("bob download status = %d, want 200", code)
	}
	// Disallowed user charlie → 403.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{"X-User-Id": "charlie"}); code != http.StatusForbidden {
		t.Errorf("charlie download status = %d, want 403", code)
	}
	// Anonymous → 403.
	if code, _ := downloadViaHTTP(t, v, resp.ID, nil); code != http.StatusForbidden {
		t.Errorf("anonymous private download status = %d, want 403", code)
	}
}

func TestEndpoint_AccessControlPaid(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("paid"), 50)
	resp := uploadViaHTTP(t, v, "paid.bin", data, map[string]string{
		"title":         "Paid",
		"type":          "video",
		"access":        "paid",
		"uploader":      "owner",
		"allowed_users": "owner",
	})

	// Premium user → 200 regardless of allowed list.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{
		"X-User-Id": "rich-user",
		"X-Premium": "true",
	}); code != http.StatusOK {
		t.Errorf("premium download status = %d, want 200", code)
	}
	// Allowed owner → 200.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{"X-User-Id": "owner"}); code != http.StatusOK {
		t.Errorf("owner download status = %d, want 200", code)
	}
	// Non-premium, non-allowed → 403.
	if code, _ := downloadViaHTTP(t, v, resp.ID, map[string]string{"X-User-Id": "freeloader"}); code != http.StatusForbidden {
		t.Errorf("freeloader download status = %d, want 403", code)
	}
}

func TestEndpoint_AccessControlDeniesStream(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("privstream"), 500)
	resp := uploadViaHTTP(t, v, "privstream.mp4", data, map[string]string{
		"title":         "Private Stream",
		"type":          "video",
		"access":        "private",
		"uploader":      "owner",
		"allowed_users": "owner",
	})

	// Disallowed user → 403 on stream too.
	req := httptest.NewRequest(http.MethodGet, "/api/content/"+resp.ID+"/stream", nil)
	req.Header.Set("X-User-Id", "stranger")
	rec := httptest.NewRecorder()
	v.HandleContentStream(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("private stream status = %d, want 403", rec.Code)
	}

	// Allowed owner → 200.
	req2 := httptest.NewRequest(http.MethodGet, "/api/content/"+resp.ID+"/stream", nil)
	req2.Header.Set("X-User-Id", "owner")
	rec2 := httptest.NewRecorder()
	v.HandleContentStream(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("owner stream status = %d, want 200", rec2.Code)
	}
}

func TestEndpoint_Delete(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("del"), 50)
	resp := uploadViaHTTP(t, v, "delete.bin", data, map[string]string{
		"title":  "Delete Me",
		"type":   "document",
		"access": "public",
	})

	// Delete it.
	req := httptest.NewRequest(http.MethodDelete, "/api/content/"+resp.ID, nil)
	rec := httptest.NewRecorder()
	v.HandleContentDelete(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", rec.Code)
	}

	// Subsequent download → 404.
	code, _ := downloadViaHTTP(t, v, resp.ID, nil)
	if code != http.StatusNotFound {
		t.Errorf("download after delete status = %d, want 404", code)
	}

	// Deleting again → 404.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/content/"+resp.ID, nil)
	rec2 := httptest.NewRecorder()
	v.HandleContentDelete(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("re-delete status = %d, want 404", rec2.Code)
	}
}

func TestEndpoint_HeadReturnsHeaders(t *testing.T) {
	v := newTestVault(t)
	data := bytes.Repeat([]byte("head"), 100)
	resp := uploadViaHTTP(t, v, "head.bin", data, map[string]string{
		"title":  "Head", "type": "document", "access": "public",
	})

	req := httptest.NewRequest(http.MethodHead, "/api/content/"+resp.ID, nil)
	rec := httptest.NewRecorder()
	v.HandleContentDownload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD body should be empty, got %d bytes", rec.Body.Len())
	}
	if rec.Header().Get("Content-Length") != strconv.Itoa(len(data)) {
		t.Errorf("Content-Length = %q, want %d", rec.Header().Get("Content-Length"), len(data))
	}
	if rec.Header().Get("X-Content-Id") != resp.ID {
		t.Errorf("X-Content-Id = %q, want %q", rec.Header().Get("X-Content-Id"), resp.ID)
	}
}

func TestEndpoint_DefaultVaultAndStandaloneHandlers(t *testing.T) {
	v := newTestVault(t)
	SetDefaultVault(v)
	if DefaultVault() == nil {
		t.Fatal("DefaultVault should be set")
	}

	// Use the standalone HandleList function.
	req := httptest.NewRequest(http.MethodGet, "/api/content", nil)
	rec := httptest.NewRecorder()
	HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HandleList status = %d", rec.Code)
	}
}

// Ensure strconv import is used in the test file (HEAD test).
var _ = strconv.Itoa
