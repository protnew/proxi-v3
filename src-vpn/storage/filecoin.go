// Package storage provides decentralized storage backends.
//
// filecoin.go implements pinning/unpinning of CIDs via a Filecoin pinning service
// (e.g., Pinata, web3.storage, or Estuary-compatible API).
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FilecoinPinner manages pinning CIDs to a Filecoin-compatible pinning service.
type FilecoinPinner struct {
	apiURL     string
	token      string
	httpClient *http.Client
}

// pinRequest is the JSON body for a pin request.
type pinRequest struct {
	CID string `json:"cid"`
}

// pinResponse is the JSON response from the pinning service.
type pinResponse struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	CID       string `json:"cid"`
}

// statusResponse is the JSON response for a status check.
type statusResponse struct {
	Status string `json:"status"`
	CID    string `json:"cid"`
}

// NewFilecoinPinner creates a new FilecoinPinner with the given API URL and auth token.
func NewFilecoinPinner(apiURL, token string) *FilecoinPinner {
	return &FilecoinPinner{
		apiURL: apiURL,
		token:  token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PinCID pins a CID to the Filecoin pinning service via POST /api/pin/{cid}.
// Returns the request ID from the pinning service on success.
func (f *FilecoinPinner) PinCID(cid string) (string, error) {
	url := f.apiURL + "/api/pin/" + cid
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	f.setAuth(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("pin CID %s: %w", cid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("pin CID %s: HTTP %d: %s", cid, resp.StatusCode, string(body))
	}

	var result pinResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode pin response: %w", err)
	}

	return result.RequestID, nil
}

// UnpinCID removes a CID from the Filecoin pinning service via DELETE /api/pin/{cid}.
func (f *FilecoinPinner) UnpinCID(cid string) error {
	url := f.apiURL + "/api/pin/" + cid
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	f.setAuth(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unpin CID %s: %w", cid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unpin CID %s: HTTP %d: %s", cid, resp.StatusCode, string(body))
	}

	return nil
}

// GetStatus checks the pin status of a CID via GET /api/pin/{cid}/status.
// Returns the status string (e.g., "pinned", "pinning", "queued", "failed").
func (f *FilecoinPinner) GetStatus(cid string) (string, error) {
	url := f.apiURL + "/api/pin/" + cid + "/status"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	f.setAuth(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get status for CID %s: %w", cid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get status for CID %s: HTTP %d: %s", cid, resp.StatusCode, string(body))
	}

	var result statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode status response: %w", err)
	}

	return result.Status, nil
}

// IsAvailable checks whether the Filecoin pinning service is reachable.
// Returns true if the health endpoint responds with HTTP 200.
func (f *FilecoinPinner) IsAvailable() bool {
	if f.apiURL == "" {
		return false
	}

	url := f.apiURL + "/api/health"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	f.setAuth(req)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// setAuth sets the Authorization header with the Bearer token.
func (f *FilecoinPinner) setAuth(req *http.Request) {
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
}

// UploadData uploads raw data to the Filecoin pinning service and returns the CID.
// This is a convenience method that POSTs data to /api/upload.
func (f *FilecoinPinner) UploadData(filename string, data []byte) (string, error) {
	url := f.apiURL + "/api/upload"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	f.setAuth(req)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Filename", filename)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload data: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}

	return result.CID, nil
}
