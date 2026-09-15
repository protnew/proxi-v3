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
	if origin := r.Header.Get("Origin"); origin != "" && !wsOriginAllowed(origin) {
		http.Error(w, `{"error":"FORBIDDEN","message":"origin not allowed"}`, http.StatusForbidden)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
		Subprotocols:   wsRequestedProtocols(r),
	})
	if err != nil {
		log.Printf("[nostr] upgrade error: %v", err)
		return
	}
	nostrRelay.HandleClient(&nhooyrWSConn{c: conn})
}

// handleNostrStats returns relay statistics.
