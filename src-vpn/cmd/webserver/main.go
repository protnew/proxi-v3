package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var distDir string

func main() {
	distDir = os.Getenv("DIST_DIR")
	if distDir == "" {
		distDir = "/app/dist"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fs := http.FileServer(http.Dir(distDir))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			handleAPI(w, r)
			return
		}

		// Set correct MIME types
		switch filepath.Ext(r.URL.Path) {
		case ".js":
			w.Header().Set("Content-Type", "application/javascript")
		case ".css":
			w.Header().Set("Content-Type", "text/css")
		case ".json":
			w.Header().Set("Content-Type", "application/json")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".html":
			w.Header().Set("Content-Type", "text/html")
		case ".wasm":
			w.Header().Set("Content-Type", "application/wasm")
		}

		// Static files with SPA fallback
		path := filepath.Join(distDir, r.URL.Path)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("🔥 Unkillable Messenger on :%s", port)
	log.Printf("   Web:  http://0.0.0.0:%s", port)
	log.Printf("   API:  http://0.0.0.0:%s/api/", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch r.URL.Path {
	case "/api/status":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "running",
			"version": "0.1.0",
			"vpn":     "ready",
		})
	case "/api/vpn/rpc":
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		if r.Method != "POST" {
			http.Error(w, `{"error":"POST only"}`, 405)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Write(handleVpnRPC(body))
	default:
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

func handleVpnRPC(request []byte) []byte {
	var req struct {
		Method string `json:"method"`
	}
	json.Unmarshal(request, &req)

	switch req.Method {
	case "get_status":
		resp, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"result": map[string]interface{}{
				"state":   "disconnected",
				"peers":   []interface{}{},
				"version": "0.1.0",
			},
		})
		return resp
	case "get_public_key":
		resp, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  map[string]string{"publicKey": "demo-key-open-app-to-generate"},
		})
		return resp
	case "start_exit_node":
		resp, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  map[string]string{"status": "sharing"},
		})
		return resp
	case "stop_exit_node":
		resp, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  map[string]string{"status": "stopped"},
		})
		return resp
	default:
		resp, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"error":   map[string]interface{}{"code": -32601, "message": fmt.Sprintf("not found: %s", req.Method)},
		})
		return resp
	}
}
