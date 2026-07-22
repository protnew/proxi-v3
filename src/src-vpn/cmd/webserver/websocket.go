package main

import (
	"log"
	"net/http"

	"github.com/unkillable-messenger/vpn/ipfs"

	"nhooyr.io/websocket"
)

var ipfsClient *ipfs.Client

type nhooyrWSConn struct {
	c *websocket.Conn
}

func (s *Server) handleNostrWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("[nostr] upgrade error: %v", err)
		return
	}
	nostrRelay.HandleClient(&nhooyrWSConn{c: conn})
}

// handleNostrStats returns relay statistics.
