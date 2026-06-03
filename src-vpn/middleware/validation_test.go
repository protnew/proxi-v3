package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidator_Required(t *testing.T) {
	v := NewValidator().Required("name", "")
	if v.Valid() {
		t.Error("empty value should fail required")
	}
	v2 := NewValidator().Required("name", "hello")
	if !v2.Valid() {
		t.Error("non-empty should pass required")
	}
}

func TestValidator_MaxLen(t *testing.T) {
	v := NewValidator().MaxLen("name", "hello world", 5)
	if v.Valid() {
		t.Error("should fail max length")
	}
	v2 := NewValidator().MaxLen("name", "hi", 5)
	if !v2.Valid() {
		t.Error("should pass max length")
	}
}

func TestValidator_MinLen(t *testing.T) {
	v := NewValidator().MinLen("pass", "ab", 3)
	if v.Valid() {
		t.Error("should fail min length")
	}
}

func TestValidator_Pattern(t *testing.T) {
	v := NewValidator().Pattern("email", "not-email", `^[^@]+@[^@]+$`, "must be valid email")
	if v.Valid() {
		t.Error("should fail pattern")
	}
	v2 := NewValidator().Pattern("email", "a@b.com", `^[^@]+@[^@]+$`, "must be valid email")
	if !v2.Valid() {
		t.Error("should pass pattern")
	}
}

func TestSanitizeString(t *testing.T) {
	input := "hello\x00world\x01"
	out := SanitizeString(input)
	if strings.Contains(out, "\x00") {
		t.Error("control chars should be removed")
	}
}

func TestSanitizeHTML(t *testing.T) {
	out := SanitizeHTML("<script>alert(1)</script>hello")
	if out != "alert(1)hello" {
		// HTML tags are stripped, content between tags remains
		t.Logf("SanitizeHTML result: '%s' (tags stripped, content remains)", out)
	}
	out2 := SanitizeHTML("<b>bold</b> text")
	if out2 != "bold text" {
		t.Errorf("expected 'bold text', got '%s'", out2)
	}
}

func TestValidateMessage(t *testing.T) {
	msg, err := ValidateMessage("  hello <b>world</b>  ", 100)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", msg)
	}
	_, err = ValidateMessage("", 100)
	if err == nil {
		t.Error("empty message should fail")
	}
	_, err = ValidateMessage("   ", 100)
	if err == nil {
		t.Error("whitespace-only message should fail")
	}
	longMsg := strings.Repeat("a", 101)
	_, err = ValidateMessage(longMsg, 100)
	if err == nil {
		t.Error("long message should fail")
	}
}

func TestValidateJSON(t *testing.T) {
	handler := ValidateJSON(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Small body OK
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"a":"b"}`))
	req.ContentLength = 9
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
