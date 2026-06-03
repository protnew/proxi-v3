package enterprise

import (
	"database/sql"

	"github.com/unkillable-messenger/vpn/store"
)

// BrandConfig defines white-label branding.
type BrandConfig struct {
	AppName      string `json:"app_name"`
	LogoURL      string `json:"logo_url"`
	PrimaryColor string `json:"primary_color"`
	AccentColor  string `json:"accent_color"`
}

// GetBrandConfig returns current branding.
func GetBrandConfig(db *store.Store) (*BrandConfig, error) {
	d := db.DB()
	config := &BrandConfig{AppName: "Proxi", PrimaryColor: "#6AB2F3"}
	var appName, logo, primary, accent string
	err := d.QueryRow("SELECT app_name, COALESCE(logo_url,''), COALESCE(primary_color,'#6AB2F3'), COALESCE(accent_color,'') FROM brand_config LIMIT 1").
		Scan(&appName, &logo, &primary, &accent)
	if err == sql.ErrNoRows {
		return config, nil
	}
	if err != nil {
		return config, nil
	}
	return &BrandConfig{AppName: appName, LogoURL: logo, PrimaryColor: primary, AccentColor: accent}, nil
}

// SetBrandConfig updates branding.
func SetBrandConfig(db *store.Store, config BrandConfig) error {
	d := db.DB()
	_, err := d.Exec("DELETE FROM brand_config")
	if err != nil {
		return err
	}
	_, err = d.Exec("INSERT INTO brand_config (app_name, logo_url, primary_color, accent_color) VALUES (?, ?, ?, ?)",
		config.AppName, config.LogoURL, config.PrimaryColor, config.AccentColor)
	return err
}
