package social

import (
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Story represents a disappearing story (24h TTL).
type Story struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	MediaURL  string `json:"media_url"`
	MediaType string `json:"media_type"` // image, video, text
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// CreateStory creates a new story that expires in 24 hours.
func CreateStory(db *store.Store, author, mediaURL, mediaType string) error {
	d := db.DB()
	now := time.Now().Unix()
	_, err := d.Exec(
		"INSERT INTO stories (author, media_url, media_type, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		author, mediaURL, mediaType, now, now+86400,
	)
	return err
}

// GetActiveStories returns all non-expired stories.
func GetActiveStories(db *store.Store) ([]Story, error) {
	d := db.DB()
	rows, err := d.Query(
		"SELECT id, author, media_url, media_type, created_at, expires_at FROM stories WHERE expires_at > ? ORDER BY created_at DESC",
		time.Now().Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stories []Story
	for rows.Next() {
		var s Story
		rows.Scan(&s.ID, &s.Author, &s.MediaURL, &s.MediaType, &s.CreatedAt, &s.ExpiresAt)
		stories = append(stories, s)
	}
	return stories, nil
}

// DeleteExpiredStories removes all expired stories.
func DeleteExpiredStories(db *store.Store) (int64, error) {
	d := db.DB()
	res, err := d.Exec("DELETE FROM stories WHERE expires_at <= ?", time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
