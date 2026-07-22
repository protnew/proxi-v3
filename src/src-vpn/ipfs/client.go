package ipfs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client is a minimal IPFS HTTP API client.
// Uses local go-ipfs or public gateway for pinning.
type Client struct {
	gateway    string
	apiURL     string
	httpClient *http.Client
}

// NewClient creates an IPFS client.
// gateway: public read URL (e.g. "https://ipfs.io/ipfs/")
// apiURL: local IPFS daemon API (e.g. "http://127.0.0.1:5001")
func NewClient(gateway, apiURL string) *Client {
	if gateway == "" {
		gateway = "https://ipfs.io/ipfs/"
	}
	if apiURL == "" {
		apiURL = "http://127.0.0.1:5001"
	}
	return &Client{
		gateway:    gateway,
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// UploadResult contains the CID and gateway URL.
type UploadResult struct {
	CID        string `json:"cid"`
	GatewayURL string `json:"gatewayUrl"`
	Size       int64  `json:"size"`
}

// UploadFile uploads a file to IPFS via the local daemon.
func (c *Client) UploadFile(filename string, data []byte) (*UploadResult, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write file data: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.apiURL+"/api/v0/add", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload to IPFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("IPFS API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse newline-delimited JSON response
	var result struct {
		Name string `json:"Name"`
		Hash string `json:"Hash"`
		Size string `json:"Size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &UploadResult{
		CID:        result.Hash,
		GatewayURL: c.gateway + result.Hash,
		Size:       parseInt64(result.Size),
	}, nil
}

// UploadBytes uploads raw bytes (no filename) to IPFS.
func (c *Client) UploadBytes(data []byte) (*UploadResult, error) {
	return c.UploadFile("data", data)
}

// DownloadFile downloads a file from IPFS by CID.
func (c *Client) DownloadFile(cid string) ([]byte, error) {
	resp, err := c.httpClient.Get(c.gateway + cid)
	if err != nil {
		return nil, fmt.Errorf("download from IPFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("IPFS gateway error %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// PinFile pins a CID to the local node.
func (c *Client) PinFile(cid string) error {
	resp, err := c.httpClient.Post(
		c.apiURL+"/api/v0/pin/add?arg="+cid,
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("pin API error %d", resp.StatusCode)
	}
	return nil
}

// IsAvailable checks if the local IPFS daemon is running.
func (c *Client) IsAvailable() bool {
	resp, err := c.httpClient.Get(c.apiURL + "/api/v0/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
