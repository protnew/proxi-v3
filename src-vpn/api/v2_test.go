package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestV2Handler_UnknownResource(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=unknown", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("should have error for unknown resource")
	}
}

func TestV2Handler_Messages(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=messages", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// No messages yet, but should not error
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestV2Handler_Channels(t *testing.T) {
	db := newTestDB(t)
	h := NewV2Handler(db)
	req := httptest.NewRequest("GET", "/api/v2?resource=channels", nil)
	w := httptest.NewRecorder()
	h.HandleQuery(w, req)
	var resp JSONAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}
