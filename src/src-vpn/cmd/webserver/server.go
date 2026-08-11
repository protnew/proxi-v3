package main

import (
	"github.com/unkillable-messenger/vpn/auth"
	"github.com/unkillable-messenger/vpn/chat"
	"github.com/unkillable-messenger/vpn/store"
)

type Server struct {
	db          *store.Store
	authService *auth.AuthService
	hub         *chat.ChatHub
	drSessions  *chat.DRSessionStore // CRYP-010 Double Ratchet sessions
}
