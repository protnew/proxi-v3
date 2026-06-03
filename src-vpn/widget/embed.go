package widget

import (
	"fmt"
)

// WidgetConfig defines embeddable chat widget settings.
type WidgetConfig struct {
	ServerURL string `json:"server_url"`
	ChannelID string `json:"channel_id"`
	Theme     string `json:"theme"` // light, dark
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// GenerateEmbedCode returns an HTML snippet for embedding.
func GenerateEmbedCode(config WidgetConfig) string {
	return fmt.Sprintf(`<div id="proxi-widget"></div>
<script src="%s/widget.js" data-channel="%s" data-theme="%s"></script>`,
		config.ServerURL, config.ChannelID, config.Theme)
}

// GenerateIframeURL returns the iframe source URL.
func GenerateIframeURL(config WidgetConfig) string {
	return fmt.Sprintf("%s/widget?channel=%s&theme=%s", config.ServerURL, config.ChannelID, config.Theme)
}
