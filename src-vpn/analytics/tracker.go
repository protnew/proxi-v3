package analytics

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Tracker records and queries analytics events.
type Tracker struct {
	db *store.Store
}

// NewTracker creates a new analytics Tracker backed by the given Store.
func NewTracker(db *store.Store) *Tracker {
	return &Tracker{db: db}
}

// TrackEvent inserts an analytics event into the database.
func (t *Tracker) TrackEvent(userID, event string, props map[string]interface{}) error {
	propsJSON := "{}"
	if props != nil {
		b, err := json.Marshal(props)
		if err != nil {
			return fmt.Errorf("marshal properties: %w", err)
		}
		propsJSON = string(b)
	}
	_, err := t.db.DB().Exec(
		`INSERT INTO analytics_events (user_id, event, properties, created_at) VALUES (?, ?, ?, ?)`,
		userID, event, propsJSON, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("track event: %w", err)
	}
	return nil
}

// GetDAU returns the number of distinct active users in the last N days.
func (t *Tracker) GetDAU(days int) (int, error) {
	since := time.Now().AddDate(0, 0, -days).Unix()
	var count int
	err := t.db.DB().QueryRow(
		`SELECT COUNT(DISTINCT user_id) FROM analytics_events WHERE created_at >= ?`,
		since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get DAU: %w", err)
	}
	return count, nil
}

// GetMAU returns the number of distinct active users in the last 30 days.
func (t *Tracker) GetMAU() (int, error) {
	return t.GetDAU(30)
}

// GetMessagesPerDay returns message counts per day for the last N days.
func (t *Tracker) GetMessagesPerDay(days int) ([]map[string]interface{}, error) {
	since := time.Now().AddDate(0, 0, -days).Unix()
	rows, err := t.db.DB().Query(
		`SELECT date(created_at, 'unixepoch') AS day, COUNT(*) AS cnt
		 FROM analytics_events
		 WHERE event = 'message_sent' AND created_at >= ?
		 GROUP BY day
		 ORDER BY day ASC`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("get messages per day: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var day string
		var cnt int
		if err := rows.Scan(&day, &cnt); err != nil {
			return nil, fmt.Errorf("scan messages per day: %w", err)
		}
		result = append(result, map[string]interface{}{
			"date":  day,
			"count": cnt,
		})
	}
	return result, rows.Err()
}

// GetRetention returns the retention rate for a given cohort date (YYYY-MM-DD).
// Cohort is the date users first appeared. Retention = fraction still active after 7 days.
func (t *Tracker) GetRetention(cohort string) (float64, error) {
	// Parse cohort date
	cohortTime, err := time.Parse("2006-01-02", cohort)
	if err != nil {
		return 0, fmt.Errorf("parse cohort date: %w", err)
	}
	cohortStart := cohortTime.Unix()
	cohortEnd := cohortTime.AddDate(0, 0, 1).Unix()
	weekLater := cohortTime.AddDate(0, 0, 7).Unix()

	// Get users who first appeared on cohort date
	rows, err := t.db.DB().Query(
		`SELECT DISTINCT user_id FROM analytics_events
		 WHERE created_at >= ? AND created_at < ?
		 GROUP BY user_id
		 HAVING MIN(created_at) >= ? AND MIN(created_at) < ?`,
		cohortStart, cohortEnd, cohortStart, cohortEnd,
	)
	if err != nil {
		return 0, fmt.Errorf("get cohort users: %w", err)
	}

	var cohortUsers []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan cohort user: %w", err)
		}
		cohortUsers = append(cohortUsers, uid)
	}
	rows.Close()

	if len(cohortUsers) == 0 {
		return 0, nil
	}

	// Count how many are still active after 7 days
	retained := 0
	for _, uid := range cohortUsers {
		var cnt int
		err := t.db.DB().QueryRow(
			`SELECT COUNT(*) FROM analytics_events WHERE user_id = ? AND created_at >= ?`,
			uid, weekLater,
		).Scan(&cnt)
		if err != nil {
			return 0, fmt.Errorf("check retained user: %w", err)
		}
		if cnt > 0 {
			retained++
		}
	}

	return float64(retained) / float64(len(cohortUsers)), nil
}
