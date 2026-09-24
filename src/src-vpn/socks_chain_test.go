package vpn

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStage0_SOCKSUsesChainNotNIC(t *testing.T) {
	var saw string
	ch := DesktopChain(map[string]func(context.Context, string) (net.Conn, error){
		"dc": func(ctx context.Context, target string) (net.Conn, error) {
			saw = target
			a, b := net.Pipe()
			b.Close()
			return a, nil
		},
	})
	s := NewSOCKS5Server("127.0.0.1:0", "")
	s.UseChain(ch)
	conn, err := s.dialTarget("1.1.1.1:443")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if saw != "1.1.1.1:443" {
		t.Fatalf("chain not used: %q", saw)
	}
}

func TestMASQUE_MetadataDenied(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "http://127.0.0.1/masque?target=169.254.169.254:80", nil)
	rec := httptest.NewRecorder()
	handleMASQUE(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
}

func TestMASQUE_PublicNotLinked(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "http://127.0.0.1/masque?target=1.1.1.1:443", nil)
	rec := httptest.NewRecorder()
	handleMASQUE(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("code %d", rec.Code)
	}
}
