package vpn

import (
	"net/http"
	"strings"
)

// masque-go has no release that pins quic-go v0.61
// (v0.4.0 wants v0.60, v0.5.0 wants v0.62). The dep is not linked.
// /masque is registered and fails closed.

func handleMASQUE(w http.ResponseWriter, r *http.Request) {
	target := r.Host
	if t := r.URL.Query().Get("target"); t != "" {
		target = t
	}
	if i := strings.Index(target, "/"); i >= 0 && !strings.Contains(target, ":") {
		target = target[:i]
	}
	if targetDenied(target) {
		http.Error(w, "masque target denied", http.StatusForbidden)
		return
	}
	http.Error(w, "masque-go not linked: no release pins quic-go v0.61", http.StatusNotImplemented)
}
