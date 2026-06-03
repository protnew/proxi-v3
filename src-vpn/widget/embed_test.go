package widget

import "testing"

func TestGenerateEmbedCode(t *testing.T) {
	config := WidgetConfig{
		ServerURL: "https://app.example.com",
		ChannelID: "ch123",
		Theme:     "dark",
		Width:     400,
		Height:    600,
	}
	code := GenerateEmbedCode(config)
	if code == "" {
		t.Error("embed code should not be empty")
	}
	t.Logf("Embed: %s", code)
}

func TestGenerateIframeURL(t *testing.T) {
	config := WidgetConfig{
		ServerURL: "https://app.example.com",
		ChannelID: "ch123",
		Theme:     "light",
	}
	url := GenerateIframeURL(config)
	if url != "https://app.example.com/widget?channel=ch123&theme=light" {
		t.Errorf("unexpected URL: %s", url)
	}
}
