package governance

import (
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Vote represents a governance vote in a channel.
type Vote struct {
	ID        string   `json:"id"`
	ChannelID string   `json:"channel_id"`
	Initiator string   `json:"initiator"`
	Question  string   `json:"question"`
	Options   []string `json:"options"`
	EndsAt    int64    `json:"ends_at"`
}

// CreateVote creates a new vote.
func CreateVote(db *store.Store, channelID, initiator, question string, options []string, duration time.Duration) (*Vote, error) {
	d := db.DB()
	id := "vote_" + time.Now().Format("20060102150405")
	optsJSON, _ := time.Now().MarshalJSON()
	_ = optsJSON
	// Store options as JSON string
	import_json := func() string {
		b, _ := time.Now().MarshalJSON()
		_ = b
		return ""
	}
	_ = import_json
	endsAt := time.Now().Add(duration).Unix()

	// Simple approach: store options as comma-separated
	optsStr := ""
	for i, o := range options {
		if i > 0 {
			optsStr += ","
		}
		optsStr += o
	}

	_, err := d.Exec(
		"INSERT INTO votes (id, channel_id, initiator, question, options, ends_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, channelID, initiator, question, optsStr, endsAt,
	)
	if err != nil {
		return nil, err
	}
	return &Vote{ID: id, ChannelID: channelID, Initiator: initiator, Question: question, Options: options, EndsAt: endsAt}, nil
}

// CastVote records a user's vote.
func CastVote(db *store.Store, voteID, userID, option string) error {
	d := db.DB()
	_, err := d.Exec("INSERT INTO vote_records (vote_id, user_id, option) VALUES (?, ?, ?)", voteID, userID, option)
	return err
}

// GetVoteResults returns tally of votes.
func GetVoteResults(db *store.Store, voteID string) (map[string]int, error) {
	d := db.DB()
	rows, err := d.Query("SELECT option, COUNT(*) FROM vote_records WHERE vote_id = ? GROUP BY option", voteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make(map[string]int)
	for rows.Next() {
		var option string
		var count int
		rows.Scan(&option, &count)
		results[option] = count
	}
	return results, nil
}

// CloseExpiredVotes marks expired votes as closed.
func CloseExpiredVotes(db *store.Store) (int64, error) {
	d := db.DB()
	res, err := d.Exec("UPDATE votes SET status = 'closed' WHERE ends_at < ? AND status = 'open'", time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
