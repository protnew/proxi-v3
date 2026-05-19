package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

const maxFileSize = 50 << 20 // 50 MB

// uploadDir returns the uploads directory path.
func uploadDir() string {
	dir := getDataDir() + "/uploads"
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// handleFileUpload handles POST /api/files/upload — multipart form upload.
func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to upload files")
		return
	}

	// Limit total request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "Failed to parse multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No file provided in 'file' field")
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > maxFileSize {
		writeError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "File exceeds 50MB limit")
		return
	}

	// Generate UUID-style file ID
	fileID := fmt.Sprintf("file-%d-%s", time.Now().UnixNano(), randomHex(8))

	// Determine content type
	contentType := detectContentType(header.Filename, file)

	// Create destination file
	destPath := filepath.Join(uploadDir(), fileID)
	dst, err := os.Create(destPath)
	if err != nil {
		log.Printf("ERROR: failed to create file %s: %v", destPath, err)
		writeError(w, http.StatusInternalServerError, "IO_ERROR", "Failed to save file")
		return
	}
	defer dst.Close()

	// Copy file content
	written, err := io.Copy(dst, file)
	if err != nil {
		log.Printf("ERROR: failed to write file %s: %v", destPath, err)
		os.Remove(destPath)
		writeError(w, http.StatusInternalServerError, "IO_ERROR", "Failed to save file")
		return
	}

	// Get uploader npub
	uploadedBy := r.FormValue("npub")
	if uploadedBy == "" {
		uploadedBy = "anonymous"
	}

	// Save metadata to DB
	fm := store.FileMeta{
		ID:         fileID,
		Name:       header.Filename,
		Size:       written,
		Type:       contentType,
		UploadedBy: uploadedBy,
		CreatedAt:  time.Now().Unix(),
	}
	if err := db.SaveFileMeta(fm); err != nil {
		log.Printf("WARNING: failed to save file metadata: %v", err)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":        fileID,
		"name":      header.Filename,
		"size":      written,
		"type":      contentType,
		"url":       "/api/files/" + fileID,
		"uploadedBy": uploadedBy,
	})

	log.Printf("📎 File uploaded: %s (%s, %d bytes) by %s", header.Filename, contentType, written, uploadedBy)
}

// handleFileGet handles GET /api/files/{id} and GET /api/files (list).
func handleFileGet(w http.ResponseWriter, r *http.Request) {
	// Apply CORS + security headers manually since this route isn't rate-limited
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Extract path after /api/files/
	path := strings.TrimPrefix(r.URL.Path, "/api/files/")

	if path == "" || r.URL.Path == "/api/files" {
		// List all files
		handleFileList(w, r)
		return
	}

	switch r.Method {
	case "GET":
		handleFileDownload(w, r, path)
	case "DELETE":
		handleFileDelete(w, r, path)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or DELETE")
	}
}

// handleFileDownload serves a file by ID.
func handleFileDownload(w http.ResponseWriter, r *http.Request, fileID string) {
	// Security check
	if containsPathTraversal(fileID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get metadata
	fm, err := db.GetFileMeta(fileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	// Serve the file
	filePath := filepath.Join(uploadDir(), fileID)
	if fm.Type != "" {
		w.Header().Set("Content-Type", fm.Type)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, fm.Name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fm.Size))
	http.ServeFile(w, r, filePath)
}

// handleFileList returns metadata for all uploaded files.
func handleFileList(w http.ResponseWriter, r *http.Request) {
	files, err := db.ListFileMeta()
	if err != nil {
		log.Printf("ERROR: db.ListFileMeta: %v", err)
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list files")
		return
	}
	if files == nil {
		files = []store.FileMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
		"count": len(files),
	})
}

// handleFileDelete removes a file by ID.
func handleFileDelete(w http.ResponseWriter, r *http.Request, fileID string) {
	if containsPathTraversal(fileID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Delete from disk
	filePath := filepath.Join(uploadDir(), fileID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("WARNING: failed to delete file %s: %v", filePath, err)
	}

	// Delete metadata
	if err := db.DeleteFileMeta(fileID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"fileId":  fileID,
	})

	log.Printf("🗑️  File deleted: %s", fileID)
}

// detectContentType determines MIME type from filename extension and file content.
func detectContentType(filename string, file io.ReadSeeker) string {
	// Try by extension first
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".weba":
		return "audio/webm"
	case ".flac":
		return "audio/flac"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/vnd.rar"
	case ".7z":
		return "application/x-7z-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	}

	// Fallback: sniff from content
	if file != nil {
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		file.Seek(0, io.SeekStart)
		if n > 0 {
			return http.DetectContentType(buf[:n])
		}
	}

	return "application/octet-stream"
}
