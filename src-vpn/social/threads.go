package social

import (
	"github.com/unkillable-messenger/vpn/store"
)

// ThreadReply represents a reply in a thread.
type ThreadReply struct {
	ReplyID string `json:"reply_id"`
	RootID  string `json:"root_id"`
}

// GetThread returns all replies for a root message.
func GetThread(db *store.Store, rootID string) ([]ThreadReply, error) {
	d := db.DB()
	rows, err := d.Query("SELECT reply_id, root_id FROM threads WHERE root_id = ? ORDER BY id", rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var replies []ThreadReply
	for rows.Next() {
		var r ThreadReply
		rows.Scan(&r.ReplyID, &r.RootID)
		replies = append(replies, r)
	}
	return replies, nil
}

// AddReply adds a reply to a thread.
func AddReply(db *store.Store, rootID, replyID string) error {
	d := db.DB()
	_, err := d.Exec("INSERT INTO threads (root_id, reply_id) VALUES (?, ?)", rootID, replyID)
	return err
}
