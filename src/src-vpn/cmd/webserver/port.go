package main

import (
	"fmt"
	"net"
	"os"
)

// resolvePort: empty PORT follows Vite/.env.example (8090), not historic 8080.
func resolvePort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8090"
	}
	return port
}

// listenLoopback binds 127.0.0.1:port. If that port is taken, recover on :0.
func listenLoopback(port string) (net.Listener, error) {
	if port == "" {
		port = "8090"
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err == nil {
		return ln, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

func portFromListener(ln net.Listener) string {
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return "0"
	}
	return port
}

func formatLoopback(port string) string {
	return fmt.Sprintf("127.0.0.1:%s", port)
}
