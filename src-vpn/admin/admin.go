package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// Panel provides admin/moderation operations.
type Panel struct {
	db *store.Store
}

// NewPanel creates an admin panel backed by the given store.
func NewPanel(s *store.Store) *Panel {
	return &Panel{db: s}
}

// GetDashboardStats returns overall platform statistics.
func (p *Panel) GetDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	d := p.db.DB()

	var msgCount, channelCount, peerCount, banCount, reportCount int
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&msgCount)
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM channels").Scan(&channelCount)
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM peers").Scan(&peerCount)
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_bans WHERE expires_at IS NULL OR expires_at > datetime('now')").Scan(&banCount)
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports WHERE status = 'open'").Scan(&reportCount)

	stats["messages"] = msgCount
	stats["channels"] = channelCount
	stats["peers"] = peerCount
	stats["active_bans"] = banCount
	stats["open_reports"] = reportCount

	// Active users (last 24h)
	var activeUsers int
	d.QueryRowContext(ctx, "SELECT COUNT(DISTINCT sender) FROM messages WHERE timestamp > strftime('%s','now','-1 day')").Scan(&activeUsers)
	stats["active_users_24h"] = activeUsers

	return stats, nil
}

// ListUsers returns a paginated list of users (derived from messages).
func (p *Panel) ListUsers(ctx context.Context, offset, limit int) ([]map[string]interface{}, error) {
	d := p.db.DB()
	rows, err := d.QueryContext(ctx,
		"SELECT sender, COUNT(*) as msg_count, MAX(timestamp) as last_active FROM messages GROUP BY sender ORDER BY last_active DESC LIMIT ? OFFSET ?",
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var userID string
		var msgCount int
		var lastActive sql.NullInt64
		if err := rows.Scan(&userID, &msgCount, &lastActive); err != nil {
			continue
		}
		entry := map[string]interface{}{
			"user_id":     userID,
			"msg_count":   msgCount,
			"last_active": lastActive.Int64,
		}
		// Check if banned
		var banCount int
		d.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM user_bans WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))",
			userID).Scan(&banCount)
		entry["banned"] = banCount > 0
		result = append(result, entry)
	}
	return result, nil
}

// BanUser adds a ban for the given user.
func (p *Panel) BanUser(ctx context.Context, userID, reason string, expiresAt time.Time) error {
	d := p.db.DB()
	var expiresStr interface{}
	if !expiresAt.IsZero() {
		expiresStr = expiresAt.Format("2006-01-02T15:04:05Z")
	}
	_, err := d.ExecContext(ctx,
		"INSERT INTO user_bans (user_id, reason, expires_at) VALUES (?, ?, ?)",
		userID, reason, expiresStr)
	return err
}

// UnbanUser removes all bans for the given user.
func (p *Panel) UnbanUser(ctx context.Context, userID string) error {
	d := p.db.DB()
	_, err := d.ExecContext(ctx, "DELETE FROM user_bans WHERE user_id = ?", userID)
	return err
}

// IsBanned checks if a user is currently banned.
func (p *Panel) IsBanned(ctx context.Context, userID string) (bool, error) {
	d := p.db.DB()
	var count int
	err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_bans WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))",
		userID).Scan(&count)
	return count > 0, err
}

// DeleteMessage removes a message by ID.
func (p *Panel) DeleteMessage(ctx context.Context, messageID string) error {
	d := p.db.DB()
	res, err := d.ExecContext(ctx, "DELETE FROM messages WHERE id = ?", messageID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found", messageID)
	}
	return nil
}

// CreateReport adds a new report for a message.
func (p *Panel) CreateReport(ctx context.Context, messageID int64, reporterID, reason string) error {
	d := p.db.DB()
	_, err := d.ExecContext(ctx,
		"INSERT INTO reports (message_id, reporter_id, reason) VALUES (?, ?, ?)",
		messageID, reporterID, reason)
	return err
}

// ListReports returns paginated reports.
func (p *Panel) ListReports(ctx context.Context, offset, limit int) ([]map[string]interface{}, error) {
	d := p.db.DB()
	rows, err := d.QueryContext(ctx,
		"SELECT id, message_id, reporter_id, reason, status, created_at, COALESCE(resolved_at,''), COALESCE(resolver_id,'') FROM reports ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id, messageID int64
		var reporterID, reason, status, createdAt, resolvedAt, resolverID string
		if err := rows.Scan(&id, &messageID, &reporterID, &reason, &status, &createdAt, &resolvedAt, &resolverID); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id":          id,
			"message_id":  messageID,
			"reporter_id": reporterID,
			"reason":      reason,
			"status":      status,
			"created_at":  createdAt,
			"resolved_at": resolvedAt,
			"resolver_id": resolverID,
		})
	}
	return result, nil
}

// ResolveReport updates a report's status.
func (p *Panel) ResolveReport(ctx context.Context, reportID int64, action, resolverID string) error {
	d := p.db.DB()
	res, err := d.ExecContext(ctx,
		"UPDATE reports SET status = ?, resolved_at = datetime('now'), resolver_id = ? WHERE id = ?",
		action, resolverID, reportID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("report %d not found", reportID)
	}
	return nil
}
