package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLightningClient(t *testing.T) {
	lc := NewLightningClient("http://localhost:10009", "mac123", "cert")
	if lc.LNDHost != "http://localhost:10009" {
		t.Errorf("host mismatch: %s", lc.LNDHost)
	}
	if lc.Macaroon != "mac123" {
		t.Errorf("macaroon mismatch: %s", lc.Macaroon)
	}
	if lc.TLSCert != "cert" {
		t.Errorf("cert mismatch: %s", lc.TLSCert)
	}
	if lc.client == nil {
		t.Error("client should not be nil")
	}
}

func TestCreateInvoice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/invoices" {
			t.Errorf("expected /v1/invoices, got %s", r.URL.Path)
		}
		if r.Header.Get("Grpc-Metadata-macaroon") != "testmac" {
			t.Errorf("macaroon header missing")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type missing")
		}
		resp := InvoiceResponse{
			RHash:          "abc123hash",
			PaymentRequest: "lnbc1000n1...",
			AddIndex:       "42",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "testmac", "")
	hash, req, err := lc.CreateInvoice(1000, "test invoice")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc123hash" {
		t.Errorf("hash mismatch: %s", hash)
	}
	if req != "lnbc1000n1..." {
		t.Errorf("payment request mismatch: %s", req)
	}
}

func TestCreateInvoice_BadURL(t *testing.T) {
	lc := NewLightningClient("http://127.0.0.1:1", "mac", "")
	lc.client.Timeout = 1 // force quick failure
	_, _, err := lc.CreateInvoice(1000, "test")
	if err == nil {
		t.Error("expected error for unreachable LND")
	}
}

func TestCreateInvoice_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	_, _, err := lc.CreateInvoice(1000, "test")
	if err == nil {
		t.Error("expected error for bad JSON response")
	}
}

func TestPayInvoice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/router/send" {
			t.Errorf("expected /v2/router/send, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	hash, err := lc.PayInvoice("0123456789abcdef0123456789abcdef_payment_req")
	if err != nil {
		t.Fatal(err)
	}
	expected := "0123456789abcdef0123456789abcdef"[0:32]
	if hash != expected {
		t.Errorf("hash mismatch: got %q, want %q", hash, expected)
	}
}

func TestPayInvoice_Unreachable(t *testing.T) {
	lc := NewLightningClient("http://127.0.0.1:1", "mac", "")
	lc.client.Timeout = 1
	_, err := lc.PayInvoice("payment_request_long_enough_for_32_chars!!")
	if err == nil {
		t.Error("expected error for unreachable LND")
	}
}

func TestCheckPayment_Settled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Grpc-Metadata-macaroon") != "testmac" {
			t.Errorf("macaroon header missing")
		}
		json.NewEncoder(w).Encode(map[string]bool{"settled": true})
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "testmac", "")
	paid, err := lc.CheckPayment("somehash")
	if err != nil {
		t.Fatal(err)
	}
	if !paid {
		t.Error("expected paid=true")
	}
}

func TestCheckPayment_NotSettled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"settled": false})
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	paid, err := lc.CheckPayment("somehash")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		t.Error("expected paid=false")
	}
}

func TestCheckPayment_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	paid, err := lc.CheckPayment("somehash")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		t.Error("expected paid=false for empty response")
	}
}

func TestCheckPayment_Unreachable(t *testing.T) {
	lc := NewLightningClient("http://127.0.0.1:1", "mac", "")
	lc.client.Timeout = 1
	_, err := lc.CheckPayment("hash")
	if err == nil {
		t.Error("expected error for unreachable LND")
	}
}

func TestIsAvailable_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/getinfo" {
			t.Errorf("expected /v1/getinfo, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	if !lc.IsAvailable() {
		t.Error("should be available")
	}
}

func TestIsAvailable_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	lc := NewLightningClient(srv.URL, "mac", "")
	if lc.IsAvailable() {
		t.Error("should not be available with 500")
	}
}
