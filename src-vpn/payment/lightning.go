package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LightningClient interacts with LND REST API for Lightning Network payments.
type LightningClient struct {
	LNDHost  string
	Macaroon string
	TLSCert  string
	client   *http.Client
}

// NewLightningClient creates a new LND client.
func NewLightningClient(host, macaroon, tlsCert string) *LightningClient {
	return &LightningClient{
		LNDHost:  host,
		Macaroon: macaroon,
		TLSCert:  tlsCert,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// InvoiceResponse represents an LND invoice.
type InvoiceResponse struct {
	RHash          string `json:"r_hash"`
	PaymentRequest string `json:"payment_request"`
	AddIndex       string `json:"add_index"`
}

// CreateInvoice creates a Lightning invoice.
func (l *LightningClient) CreateInvoice(amountSats int64, description string) (paymentHash, paymentRequest string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"value":        amountSats,
		"memo":         description,
		"expiry":       3600,
		"private":      true,
	})
	req, err := http.NewRequest("POST", l.LNDHost+"/v1/invoices", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("lnd request: %w", err)
	}
	defer resp.Body.Close()

	var inv InvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return "", "", fmt.Errorf("decode invoice: %w", err)
	}
	return inv.RHash, inv.PaymentRequest, nil
}

// PayInvoice pays a Lightning invoice.
func (l *LightningClient) PayInvoice(paymentRequest string) (paymentHash string, err error) {
	body, _ := json.Marshal(map[string]string{"payment_request": paymentRequest})
	req, _ := http.NewRequest("POST", l.LNDHost+"/v2/router/send", bytes.NewReader(body))
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lnd pay: %w", err)
	}
	defer resp.Body.Close()
	return paymentRequest[:32], nil
}

// CheckPayment checks if an invoice has been paid.
func (l *LightningClient) CheckPayment(paymentHash string) (paid bool, err error) {
	req, _ := http.NewRequest("GET", l.LNDHost+"/v1/invoice/"+paymentHash, nil)
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	resp, err := l.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	settled, _ := result["settled"].(bool)
	return settled, nil
}

// IsAvailable checks if LND node is reachable.
func (l *LightningClient) IsAvailable() bool {
	req, _ := http.NewRequest("GET", l.LNDHost+"/v1/getinfo", nil)
	req.Header.Set("Grpc-Metadata-macaroon", l.Macaroon)
	resp, err := l.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
