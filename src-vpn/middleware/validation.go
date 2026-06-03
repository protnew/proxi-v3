package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validator provides input validation helpers.
type Validator struct {
	errors []string
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Required checks that a string field is non-empty.
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s is required", field))
	}
	return v
}

// MaxLen checks max UTF-8 length.
func (v *Validator) MaxLen(field, value string, max int) *Validator {
	if utf8.RuneCountInString(value) > max {
		v.errors = append(v.errors, fmt.Sprintf("%s exceeds %d chars", field, max))
	}
	return v
}

// MinLen checks min UTF-8 length.
func (v *Validator) MinLen(field, value string, min int) *Validator {
	if utf8.RuneCountInString(value) < min {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at least %d chars", field, min))
	}
	return v
}

// Pattern checks a regex match.
func (v *Validator) Pattern(field, value, pattern, msg string) *Validator {
	re := regexp.MustCompile(pattern)
	if !re.MatchString(value) {
		v.errors = append(v.errors, fmt.Sprintf("%s %s", field, msg))
	}
	return v
}

// Valid returns true if no validation errors.
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Errors returns validation error messages.
func (v *Validator) Errors() []string {
	return v.errors
}

// SanitizeString removes control characters and trims whitespace.
func SanitizeString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 32 || r == 10 || r == 9 { // 10 = newline, 9 = tab
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// SanitizeHTML strips all HTML tags.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func SanitizeHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

// ValidateJSON is middleware that validates JSON body size.
func ValidateJSON(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("request body too large (max %d bytes)", maxBytes),
				})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateMessage validates a chat message before storage.
func ValidateMessage(text string, maxLen int) (string, error) {
	text = SanitizeString(text)
	text = SanitizeHTML(text)
	if utf8.RuneCountInString(text) == 0 {
		return "", fmt.Errorf("message cannot be empty")
	}
	if utf8.RuneCountInString(text) > maxLen {
		return "", fmt.Errorf("message exceeds %d chars", maxLen)
	}
	return text, nil
}
