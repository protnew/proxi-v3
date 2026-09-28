package analytics

import (
	"testing"
)

func TestTrackEvent(t *testing.T) {
	m := NewManager()
	m.Track("view", "user1", "post1", "ch1", nil)
	m.Track("like", "user1", "post1", "ch1", nil)

	if m.EventCount() != 2 { t.Fatalf("events: got %d", m.EventCount()) }
}

func TestChannelAnalytics(t *testing.T) {
	m := NewManager()

	// Simulate activity
	for i := 0; i < 100; i++ {
		m.Track("view", "user1", "post1", "ch1", map[string]interface{}{"title": "Best Post"})
	}
	for i := 0; i < 20; i++ {
		m.Track("like", "user1", "post1", "ch1", nil)
	}
	m.Track("subscribe", "user2", "", "ch1", nil)
	m.Track("subscribe", "user3", "", "ch1", nil)
	m.Track("donate", "user2", "post1", "ch1", map[string]interface{}{"amount": 500.0})
	m.Track("donate", "user3", "post1", "ch1", map[string]interface{}{"amount": 300.0})
	m.Track("share", "user1", "post1", "ch1", nil)

	stats := m.GetChannelAnalytics("ch1")
	if stats.TotalViews != 100 { t.Fatalf("views: got %d", stats.TotalViews) }
	if stats.TotalLikes != 20 { t.Fatalf("likes: got %d", stats.TotalLikes) }
	if stats.TotalSubscribers != 2 { t.Fatalf("subs: got %d", stats.TotalSubscribers) }
	if stats.TotalDonations != 800 { t.Fatalf("donations: got %.2f", stats.TotalDonations) }
	if len(stats.TopPosts) != 1 { t.Fatalf("top posts: got %d", len(stats.TopPosts)) }
	if stats.TopPosts[0].Title != "Best Post" { t.Fatal("wrong title") }
}

func TestDailyStats(t *testing.T) {
	m := NewManager()
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Track("like", "u1", "p1", "ch1", nil)

	stats := m.GetChannelAnalytics("ch1")
	if len(stats.Daily) != 1 { t.Fatalf("days: got %d", len(stats.Daily)) }
	if stats.Daily[0].Views != 2 { t.Fatalf("daily views: got %d", stats.Daily[0].Views) }
}

func TestFilterByType(t *testing.T) {
	m := NewManager()
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Track("like", "u1", "p1", "ch1", nil)
	m.Track("view", "u2", "p2", "ch1", nil)

	views := m.GetEventsByType("view")
	if len(views) != 2 { t.Fatalf("views: got %d", len(views)) }
}

func TestFilterByUser(t *testing.T) {
	m := NewManager()
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Track("view", "u2", "p2", "ch1", nil)
	m.Track("like", "u1", "p1", "ch1", nil)

	events := m.GetEventsByUser("u1")
	if len(events) != 2 { t.Fatalf("u1 events: got %d", len(events)) }
}

func TestEmptyAnalytics(t *testing.T) {
	m := NewManager()
	stats := m.GetChannelAnalytics("ch1")
	if stats.TotalViews != 0 { t.Fatal("should be 0") }
	if len(stats.TopPosts) != 0 { t.Fatal("should be empty") }
}

func TestClear(t *testing.T) {
	m := NewManager()
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Clear()
	if m.EventCount() != 0 { t.Fatalf("events after clear: got %d", m.EventCount()) }
}

func TestDifferentChannels(t *testing.T) {
	m := NewManager()
	m.Track("view", "u1", "p1", "ch1", nil)
	m.Track("view", "u1", "p1", "ch2", nil)
	m.Track("view", "u1", "p1", "ch2", nil)

	stats1 := m.GetChannelAnalytics("ch1")
	stats2 := m.GetChannelAnalytics("ch2")
	if stats1.TotalViews != 1 { t.Fatalf("ch1 views: got %d", stats1.TotalViews) }
	if stats2.TotalViews != 2 { t.Fatalf("ch2 views: got %d", stats2.TotalViews) }
}
