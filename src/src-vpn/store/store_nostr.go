package store

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NostrEvent represents a Nostr event.
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
	CreatedAt int64      `json:"created_at"`
}

// SaveNostrEvent persists a Nostr event. Uses INSERT OR IGNORE to skip duplicates.
func (s *Store) SaveNostrEvent(evt NostrEvent) error {
	tagsJSON, err := json.Marshal(evt.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO nostr_events (id, pubkey, kind, tags, content, sig, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		evt.ID, evt.PubKey, evt.Kind, string(tagsJSON), evt.Content, evt.Sig, evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save nostr event %s: %w", evt.ID, err)
	}
	return nil
}

// NostrEventFilter holds filter parameters for querying events.
type NostrEventFilter struct {
	Kinds   []int
	Authors []string
	Since   *int64
	Until   *int64
	Limit   int
	IDs     []string
}

// GetNostrEvents retrieves events matching the given filter.
// Results are ordered by created_at DESC (newest first).
func (s *Store) GetNostrEvents(filter NostrEventFilter) ([]NostrEvent, error) {
	var conditions []string
	var args []interface{}

	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "id IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Authors) > 0 {
		placeholders := make([]string, len(filter.Authors))
		for i, a := range filter.Authors {
			placeholders[i] = "?"
			args = append(args, a)
		}
		conditions = append(conditions, "pubkey IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Kinds) > 0 {
		placeholders := make([]string, len(filter.Kinds))
		for i, k := range filter.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.Since)
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.Until)
	}

	query := "SELECT id, pubkey, kind, tags, content, sig, created_at FROM nostr_events"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get nostr events: %w", err)
	}
	defer rows.Close()

	var events []NostrEvent
	for rows.Next() {
		var evt NostrEvent
		var tagsJSON string
		if err := rows.Scan(&evt.ID, &evt.PubKey, &evt.Kind, &tagsJSON, &evt.Content, &evt.Sig, &evt.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan nostr event: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &evt.Tags); err != nil {
			// If tags are malformed, skip them
			evt.Tags = nil
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

// DeleteOldNostrEvents removes events older than the given unix timestamp.
// Returns the number of deleted events.
func (s *Store) DeleteOldNostrEvents(olderThan int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM nostr_events WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete old nostr events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
