package analytics

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Event represents an analytics event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "view", "like", "subscribe", "donate", "share"
	UserID    string                 `json:"user_id"`
	ContentID string                 `json:"content_id"`
	ChannelID string                 `json:"channel_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp int64                  `json:"ts"`
}

// DailyStats aggregates stats for a day
type DailyStats struct {
	Date         string  `json:"date"`
	Views        int     `json:"views"`
	Likes        int     `json:"likes"`
	Subscribers  int     `json:"subscribers"`
	Donations    int     `json:"donations"`
	DonationSum  float64 `json:"donation_sum"`
	Shares       int     `json:"shares"`
	Unsubscribes int     `json:"unsubscribes"`
}

// ChannelAnalytics is a full analytics report for a channel
type ChannelAnalytics struct {
	ChannelID       string       `json:"channel_id"`
	TotalViews      int          `json:"total_views"`
	TotalLikes      int          `json:"total_likes"`
	TotalSubscribers int         `json:"total_subscribers"`
	TotalDonations  float64      `json:"total_donations"`
	TopPosts        []PostStats  `json:"top_posts"`
	Daily           []DailyStats `json:"daily"`
	Growth7d        float64      `json:"growth_7d"` // % growth in 7 days
	UpdatedAt       int64        `json:"updated_at"`
}

// PostStats for individual posts
type PostStats struct {
	PostID   string  `json:"post_id"`
	Title    string  `json:"title"`
	Views    int     `json:"views"`
	Likes    int     `json:"likes"`
	Donations float64 `json:"donations"`
}

// Manager handles analytics
type Manager struct {
	mu     sync.RWMutex
	events []*Event
}

// NewManager creates an analytics manager
func NewManager() *Manager {
	return &Manager{
		events: make([]*Event, 0),
	}
}

// Track records an analytics event
func (m *Manager) Track(eventType, userID, contentID, channelID string, metadata map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("evt_%d", time.Now().UnixNano())
	m.events = append(m.events, &Event{
		ID:        id,
		Type:      eventType,
		UserID:    userID,
		ContentID: contentID,
		ChannelID: channelID,
		Metadata:  metadata,
		Timestamp: time.Now().Unix(),
	})
}

// GetChannelAnalytics generates analytics for a channel
func (m *Manager) GetChannelAnalytics(channelID string) *ChannelAnalytics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ChannelAnalytics{
		ChannelID: channelID,
		UpdatedAt: time.Now().Unix(),
	}

	postViews := make(map[string]int)
	postLikes := make(map[string]int)
	postDonations := make(map[string]float64)
	postTitles := make(map[string]string)
	dailyMap := make(map[string]*DailyStats)

	for _, e := range m.events {
		if e.ChannelID != channelID {
			continue
		}

		day := time.Unix(e.Timestamp, 0).Format("2006-01-02")
		if dailyMap[day] == nil {
			dailyMap[day] = &DailyStats{Date: day}
		}
		ds := dailyMap[day]

		switch e.Type {
		case "view":
			stats.TotalViews++
			ds.Views++
			postViews[e.ContentID]++
			if title, ok := e.Metadata["title"].(string); ok {
				postTitles[e.ContentID] = title
			}
		case "like":
			stats.TotalLikes++
			ds.Likes++
			postLikes[e.ContentID]++
		case "subscribe":
			stats.TotalSubscribers++
			ds.Subscribers++
		case "unsubscribe":
			ds.Unsubscribes++
		case "donate":
			if amount, ok := e.Metadata["amount"].(float64); ok {
				stats.TotalDonations += amount
				ds.Donations++
				ds.DonationSum += amount
				postDonations[e.ContentID] += amount
			}
		case "share":
			ds.Shares++
		}
	}

	// Build daily stats sorted by date
	for _, ds := range dailyMap {
		stats.Daily = append(stats.Daily, *ds)
	}
	sort.Slice(stats.Daily, func(i, j int) bool {
		return stats.Daily[i].Date < stats.Daily[j].Date
	})

	// Build top posts
	for postID := range postViews {
		title := postTitles[postID]
		if title == "" {
			title = postID
		}
		stats.TopPosts = append(stats.TopPosts, PostStats{
			PostID:    postID,
			Title:     title,
			Views:     postViews[postID],
			Likes:     postLikes[postID],
			Donations: postDonations[postID],
		})
	}
	sort.Slice(stats.TopPosts, func(i, j int) bool {
		return stats.TopPosts[i].Views > stats.TopPosts[j].Views
	})
	if len(stats.TopPosts) > 10 {
		stats.TopPosts = stats.TopPosts[:10]
	}

	return stats
}

// EventCount returns total tracked events
func (m *Manager) EventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

// GetEventsByType returns events of a specific type
func (m *Manager) GetEventsByType(eventType string) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Event
	for _, e := range m.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// GetEventsByUser returns events for a specific user
func (m *Manager) GetEventsByUser(userID string) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Event
	for _, e := range m.events {
		if e.UserID == userID {
			result = append(result, e)
		}
	}
	return result
}

// Clear removes all events (for testing)
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}
