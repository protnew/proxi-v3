package cdn

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// CDNManager manages Cloudflare CDN operations (cache purge, R2 upload).
type CDNManager struct {
	apiToken string
	zoneID   string
	baseURL  string
	r2URL    string
	client   *http.Client
}

// NewCDNManager creates a new CDNManager with the given API token and zone ID.
func NewCDNManager(apiToken, zoneID string) *CDNManager {
	return &CDNManager{
		apiToken: apiToken,
		zoneID:   zoneID,
		baseURL:  "https://api.cloudflare.com/client/v4/zones/" + zoneID,
		r2URL:    "https://" + zoneID + ".r2.cloudflarestorage.com",
		client:   &http.Client{},
	}
}

// PurgeCache purges the specified URLs from the Cloudflare cache.
func (c *CDNManager) PurgeCache(urls []string) error {
	if c.apiToken == "" {
		return fmt.Errorf("cdn: api token not configured")
	}

	body := fmt.Sprintf(`{"files":%q}`, urls)
	req, err := http.NewRequest("POST", c.baseURL+"/purge_cache", bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("cdn purge cache: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("cdn purge cache request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cdn purge cache: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UploadFile uploads data to Cloudflare R2 storage and returns the public URL.
func (c *CDNManager) UploadFile(key string, data []byte) (string, error) {
	if c.apiToken == "" {
		return "", fmt.Errorf("cdn: api token not configured")
	}

	uploadURL := c.r2URL + "/" + key
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cdn upload: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cdn upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cdn upload: status %d: %s", resp.StatusCode, string(respBody))
	}

	return c.GetURL(key), nil
}

// GetURL returns the public CDN URL for a given key.
func (c *CDNManager) GetURL(key string) string {
	return fmt.Sprintf("https://cdn.unkillable-messenger.com/%s", key)
}
