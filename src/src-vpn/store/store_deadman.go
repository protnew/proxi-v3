package store

import (
	"fmt"
	"time"
)

// ScheduledMessage represents a pending message to be sent at a future time.

type DeadMansSwitch struct {
	ID           string `json:"id"`
	UserNpub     string `json:"userNpub"`
	MessageText  string `json:"messageText"`
	Recipient    string `json:"recipient"`
	IntervalDays int    `json:"intervalDays"`
	LastCheckIn  int64  `json:"lastCheckIn"`
	Triggered    bool   `json:"triggered"`
	CreatedAt    int64  `json:"createdAt"`
}

// SaveDeadMansSwitch persists a dead man's switch configuration.

func (s *Store) SaveDeadMansSwitch(dms DeadMansSwitch) error {
	triggered := 0
	if dms.Triggered {
		triggered = 1
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO dead_mans_switch (id, user_npub, message_text, recipient, interval_days, last_check_in, triggered, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		dms.ID, dms.UserNpub, dms.MessageText, dms.Recipient, dms.IntervalDays, dms.LastCheckIn, triggered, dms.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save dead mans switch %s: %w", dms.ID, err)
	}
	return nil
}

// CheckInDeadMansSwitch updates the last_check_in timestamp for all switches of a user.

func (s *Store) CheckInDeadMansSwitch(userNpub string) error {
	_, err := s.db.Exec(
		`UPDATE dead_mans_switch SET last_check_in = ? WHERE user_npub = ?`,
		time.Now().Unix(), userNpub,
	)
	return err
}

// GetExpiredSwitches returns all switches that have exceeded their interval.

func (s *Store) GetExpiredSwitches() ([]DeadMansSwitch, error) {
	now := time.Now().Unix()
	rows, err := s.db.Query(
		`SELECT id, user_npub, message_text, recipient, interval_days, last_check_in, triggered, created_at
		 FROM dead_mans_switch
		 WHERE triggered = 0 AND (last_check_in + interval_days * 86400) < ?`, now,
	)
	if err != nil {
		return nil, fmt.Errorf("get expired switches: %w", err)
	}
	defer rows.Close()

	var switches []DeadMansSwitch
	for rows.Next() {
		var dms DeadMansSwitch
		var triggered int
		if err := rows.Scan(&dms.ID, &dms.UserNpub, &dms.MessageText, &dms.Recipient,
			&dms.IntervalDays, &dms.LastCheckIn, &triggered, &dms.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan switch: %w", err)
		}
		dms.Triggered = triggered != 0
		switches = append(switches, dms)
	}
	return switches, rows.Err()
}

// MarkSwitchTriggered marks a switch as triggered.

func (s *Store) MarkSwitchTriggered(id string) error {
	_, err := s.db.Exec(`UPDATE dead_mans_switch SET triggered = 1 WHERE id = ?`, id)
	return err
}
