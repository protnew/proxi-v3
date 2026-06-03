package payment

import (
	"github.com/unkillable-messenger/vpn/store"
)

// CreateDonation records a donation to a channel.
func CreateDonation(db *store.Store, fromUser, toChannel string, amountSats int64) error {
	d := db.DB()
	_, err := d.Exec(
		"INSERT INTO donations (from_user, to_channel, amount_sats) VALUES (?, ?, ?)",
		fromUser, toChannel, amountSats,
	)
	return err
}

// GetDonations returns donations for a channel.
func GetDonations(db *store.Store, channel string, limit int) ([]map[string]interface{}, error) {
	d := db.DB()
	rows, err := d.Query(
		"SELECT id, from_user, to_channel, amount_sats, created_at FROM donations WHERE to_channel = ? ORDER BY created_at DESC LIMIT ?",
		channel, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id int64
		var fromUser, toChannel, createdAt string
		var amountSats int64
		rows.Scan(&id, &fromUser, &toChannel, &amountSats, &createdAt)
		result = append(result, map[string]interface{}{
			"id": id, "from_user": fromUser, "to_channel": toChannel,
			"amount_sats": amountSats, "created_at": createdAt,
		})
	}
	return result, nil
}

// GetTotalDonations returns total sats donated to a channel.
func GetTotalDonations(db *store.Store, channel string) (int64, error) {
	d := db.DB()
	var total int64
	err := d.QueryRow("SELECT COALESCE(SUM(amount_sats),0) FROM donations WHERE to_channel = ?", channel).Scan(&total)
	return total, err
}
