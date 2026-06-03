package enterprise

import (
	"encoding/json"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// AuditLogger records compliance audit events.
type AuditLogger struct {
	db *store.Store
}

// NewAuditLogger creates an audit logger.
func NewAuditLogger(db *store.Store) *AuditLogger {
	return &AuditLogger{db: db}
}

// Log records an audit event.
func (a *AuditLogger) Log(userID, action, resource string, details map[string]interface{}) error {
	d := a.db.DB()
	detailsJSON, _ := json.Marshal(details)
	_, err := d.Exec(
		"INSERT INTO audit_log (user_id, action, resource, details) VALUES (?, ?, ?, ?)",
		userID, action, resource, string(detailsJSON),
	)
	return err
}

// GetLogs retrieves audit logs with filters.
func (a *AuditLogger) GetLogs(userID string, limit int) ([]map[string]interface{}, error) {
	d := a.db.DB()
	query := "SELECT id, user_id, action, resource, details, created_at FROM audit_log"
	args := []interface{}{}
	if userID != "" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id int64
		var uid, action, resource, details, createdAt string
		rows.Scan(&id, &uid, &action, &resource, &details, &createdAt)
		result = append(result, map[string]interface{}{
			"id": id, "user_id": uid, "action": action,
			"resource": resource, "details": details, "created_at": createdAt,
		})
	}
	return result, nil
}

// RetentionPolicy defines data retention rules.
type RetentionPolicy struct {
	MessageTTL    int // days, 0 = forever
	FileTTL       int // days, 0 = forever
	DeleteOnExpiry bool
}

// ApplyRetentionPolicy removes old data according to policy.
func ApplyRetentionPolicy(db *store.Store, policy RetentionPolicy) error {
	d := db.DB()
	if policy.MessageTTL > 0 {
		cutoff := time.Now().AddDate(0, 0, -policy.MessageTTL).Unix()
		d.Exec("DELETE FROM messages WHERE timestamp < ?", cutoff)
	}
	return nil
}
