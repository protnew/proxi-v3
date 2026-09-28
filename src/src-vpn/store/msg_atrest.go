package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unkillable-messenger/vpn/crypto"
)

const enc2Prefix = "enc2:"

func messageKeyPathByID(id string) (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "proxi", "keys", "msg-"+id+".key"), nil
}

func (s *Store) bindMessageKey(dbPath string) error {
	if hexKey := strings.TrimSpace(os.Getenv("PROXI_MSG_KEY")); hexKey != "" {
		raw, err := hex.DecodeString(hexKey)
		if err != nil || len(raw) != 32 {
			return fmt.Errorf("PROXI_MSG_KEY must be 64 hex chars")
		}
		copy(s.msgKey[:], raw)
		return nil
	}
	if dbPath == "" || dbPath == ":memory:" {
		_, err := rand.Read(s.msgKey[:])
		return err
	}
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS atrest_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		return err
	}
	var id string
	err := s.db.QueryRow(`SELECT v FROM atrest_meta WHERE k = 'key_id'`).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err != nil || id == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		id = hex.EncodeToString(buf)
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO atrest_meta(k, v) VALUES ('key_id', ?)`, id); err != nil {
			return err
		}
		return s.writeFreshMessageKey(id)
	}
	return s.loadMessageKeyFile(id)
}

func (s *Store) loadMessageKeyFile(id string) error {
	path, err := messageKeyPathByID(id)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("message key file missing/corrupt for key_id=%s", id)
	}
	raw, err := unprotectKey(data)
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("message key file missing/corrupt for key_id=%s", id)
	}
	copy(s.msgKey[:], raw)
	return nil
}

func (s *Store) writeFreshMessageKey(id string) error {
	path, err := messageKeyPathByID(id)
	if err != nil {
		return err
	}
	if _, err := rand.Read(s.msgKey[:]); err != nil {
		return err
	}
	blob, err := protectKey(s.msgKey[:])
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(path, blob, 0600); err != nil {
		_, _ = s.db.Exec(`DELETE FROM atrest_meta WHERE k = 'key_id' AND v = ?`, id)
		return err
	}
	return nil
}

func (s *Store) sealColumn(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, enc2Prefix) {
		return plain, nil
	}
	enc, err := crypto.EncryptMessage(plain, s.msgKey)
	if err != nil {
		return "", fmt.Errorf("seal column: %w", err)
	}
	return enc2Prefix + enc, nil
}

func (s *Store) openColumn(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, enc2Prefix) {
		return stored, nil
	}
	plain, err := crypto.DecryptMessage(strings.TrimPrefix(stored, enc2Prefix), s.msgKey)
	if err != nil {
		return "", fmt.Errorf("at-rest key mismatch")
	}
	return plain, nil
}

func (s *Store) reveal(m *Message) error {
	rawText, rawAtt := m.Text, m.Attachments
	text, err := s.openColumn(m.Text)
	if err != nil {
		return err
	}
	att, err := s.openColumn(m.Attachments)
	if err != nil {
		return err
	}
	m.Text, m.Attachments = text, att
	if (rawText != "" && !strings.HasPrefix(rawText, enc2Prefix)) || (rawAtt != "" && !strings.HasPrefix(rawAtt, enc2Prefix)) {
		sealedText, err := s.sealColumn(text)
		if err != nil {
			return err
		}
		sealedAtt, err := s.sealColumn(att)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(`UPDATE messages SET text = ?, attachments = ? WHERE id = ?`, sealedText, sealedAtt, m.ID)
		return err
	}
	return nil
}

func (s *Store) persistSealed(msg Message) error {
	if len(msg.Text) == 0 || len(strings.TrimSpace(msg.Text)) == 0 {
		return nil
	}
	if len(msg.Text) > MaxMessageLen {
		return fmt.Errorf("message too long: %d > %d", len(msg.Text), MaxMessageLen)
	}
	text, err := s.sealColumn(msg.Text)
	if err != nil {
		return err
	}
	att, err := s.sealColumn(msg.Attachments)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO messages (id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl, group_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.From, msg.To, text, boolToInt(msg.Encrypted), msg.Timestamp,
		msg.ReplyTo, msg.ForwardedFrom, att, msg.TTL, msg.Group,
	)
	if err != nil {
		return fmt.Errorf("save message %s: %w", msg.ID, err)
	}
	return nil
}

func (s *Store) editSealed(messageID, newText, senderNpub string) error {
	sealed, err := s.sealColumn(newText)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE messages SET text = ? WHERE id = ? AND sender = ? AND IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0`,
		sealed, messageID, senderNpub,
	)
	if err != nil {
		return fmt.Errorf("edit message %s: %w", messageID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found or unauthorized", messageID)
	}
	return nil
}

func (s *Store) searchOpened(query, npub string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	capN := limit * 20
	if capN < 200 {
		capN = 200
	}
	if capN > 2000 {
		capN = 2000
	}
	q := `SELECT id, sender, recipient, text, encrypted, timestamp, reply_to, forwarded_from, attachments, ttl, IFNULL(group_id, '')
		FROM messages WHERE IFNULL(is_deleted, 0) = 0 AND IFNULL(deleted_at, 0) = 0`
	args := []any{}
	if npub != "" {
		q += ` AND (recipient = 'broadcast' OR sender = ? OR recipient = ?)`
		args = append(args, npub, npub)
	}
	q += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, capN)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	needle := strings.ToLower(query)
	var msgs []Message
	for rows.Next() {
		var m Message
		var enc int
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.Text, &enc, &m.Timestamp, &m.ReplyTo, &m.ForwardedFrom, &m.Attachments, &m.TTL, &m.Group); err != nil {
			return nil, err
		}
		m.Encrypted = enc != 0
		if err := s.reveal(&m); err != nil {
			return nil, err
		}
		if needle != "" && !strings.Contains(strings.ToLower(m.Text), needle) {
			continue
		}
		msgs = append(msgs, m)
		if len(msgs) >= limit {
			break
		}
	}
	return msgs, rows.Err()
}

func (s *Store) vacuumPlaintextColumns() error {
	rows, err := s.db.Query(`SELECT id, text, IFNULL(attachments, '') FROM messages WHERE IFNULL(erased_at, 0) = 0 AND (text NOT LIKE 'enc2:%' OR (attachments != '' AND attachments NOT LIKE 'enc2:%'))`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	type row struct{ id, text, att string }
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.text, &r.att); err != nil {
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range pending {
		text, err := s.sealColumn(r.text)
		if err != nil {
			return err
		}
		att, err := s.sealColumn(r.att)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(`UPDATE messages SET text = ?, attachments = ? WHERE id = ?`, text, att, r.id); err != nil {
			return err
		}
	}
	return nil
}
