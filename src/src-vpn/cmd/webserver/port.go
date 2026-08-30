package main

import "os"

// resolvePort: empty PORT follows Vite/.env.example (8090), not historic 8080.
func resolvePort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8090"
	}
	return port
}
