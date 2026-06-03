package store

import (
	"strings"
)

// SearchMessagesFTS performs full-text search using FTS5.
// Falls back to LIKE if FTS5 table does not exist.
func (s *Store) SearchMessagesFTS(query string, limit int) ([]Message, error) {
	d := s.DB()
	cleanQuery := sanitizeFTSQuery(query)
	if cleanQuery == "" {
		return nil, nil
	}

	// Try FTS5 first
	rows, err := d.Query(`
		SELECT m.id, m.sender, m.recipient, m.text, m.timestamp, m.encrypted
		FROM messages m
		JOIN messages_fts f ON m.id = f.rowid
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, cleanQuery, limit)
	if err != nil {
		// FTS5 table may not exist, fall back to LIKE
		return s.SearchMessages(query, "", limit)
	}
	defer rows.Close()

	var results []Message
	for rows.Next() {
		var msg Message
		var enc int
		rows.Scan(&msg.ID, &msg.From, &msg.To, &msg.Text, &msg.Timestamp, &enc)
		msg.Encrypted = enc == 1
		results = append(results, msg)
	}
	return results, nil
}

// SearchChannelsFTS searches channels by name/description using FTS5.
func (s *Store) SearchChannelsFTS(query string, limit int) ([]Channel, error) {
	d := s.DB()
	cleanQuery := sanitizeFTSQuery(query)
	if cleanQuery == "" {
		return nil, nil
	}

	rows, err := d.Query(`
		SELECT id, name, description, creator, subscribers, created_at
		FROM channels
		WHERE name LIKE ? OR description LIKE ?
		ORDER BY subscribers DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Channel
	for rows.Next() {
		var ch Channel
		rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Creator, &ch.Subscribers, &ch.CreatedAt)
		results = append(results, ch)
	}
	return results, nil
}

// sanitizeFTSQuery cleans a user query for FTS5 MATCH syntax.
func sanitizeFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	// Remove special FTS5 characters
	replacer := strings.NewReplacer(
		`"`, ``, `{`, ``, `}`, ``, `(`, ``, `)`, ``,
		`*`, ``, `:`, ``, `^`, ``, `+`, ``,
	)
	query = replacer.Replace(query)

	// Split into words and join with AND
	words := strings.Fields(query)
	var clean []string
	for _, w := range words {
		if len(w) > 0 {
			clean = append(clean, w+"*")
		}
	}
	return strings.Join(clean, " AND ")
}
