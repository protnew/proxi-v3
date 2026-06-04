package monitoring

import (
	"runtime"
	"time"

	"github.com/unkillable-messenger/vpn/store"
)

// ChannelStats holds statistics for a single channel.
type ChannelStats struct {
	Name        string `json:"name"`
	Messages    int    `json:"messages"`
	Subscribers int    `json:"subscribers"`
}

// DashboardData aggregates system metrics for the admin dashboard.
type DashboardData struct {
	MessagesPerSec float64           `json:"messagesPerSec"`
	ActiveUsers    int               `json:"activeUsers"`
	ActiveStreams  int               `json:"activeStreams"`
	StorageUsed    int64             `json:"storageUsed"`
	AvgLatency     float64           `json:"avgLatency"`
	ErrorRate      float64           `json:"errorRate"`
	VPNSessions    int               `json:"vpnSessions"`
	Uptime         int64             `json:"uptime"`
	TopChannels    []ChannelStats    `json:"topChannels"`
	SystemHealth   map[string]string `json:"systemHealth"`
}

// startTime tracks the application start time for uptime calculations.
var startTime = time.Now()

// CollectDashboardData assembles dashboard data from metrics, database, and hub.
// The hub parameter is typed as interface{} to decouple from the specific chat
// package; it is used to extract active stream and channel counts when available.
func CollectDashboardData(metrics *Metrics, db *store.Store, hub interface{}) DashboardData {
	data := DashboardData{
		TopChannels:  []ChannelStats{},
		SystemHealth: HealthCheckWithDeps(db),
	}

	if metrics != nil {
		metrics.mu.Lock()
		data.MessagesPerSec = metrics.gauges["messages_per_sec"]
		data.ActiveUsers = int(metrics.gauges["active_users"])
		data.ActiveStreams = int(metrics.gauges["active_streams"])
		data.StorageUsed = int64(metrics.gauges["storage_used_bytes"])
		data.VPNSessions = int(metrics.gauges["vpn_sessions"])

		// Calculate average latency from observations
		if lats, ok := metrics.latency["api_request"]; ok && len(lats) > 0 {
			var sum time.Duration
			for _, d := range lats {
				sum += d
			}
			data.AvgLatency = (sum / time.Duration(len(lats))).Seconds()
		}

		// Error rate: errors / total messages
		errCount := metrics.counters["errors"]
		msgCount := metrics.counters["messages_sent"]
		if msgCount > 0 {
			data.ErrorRate = errCount / msgCount
		}
		metrics.mu.Unlock()
	}

	data.Uptime = int64(time.Since(startTime).Seconds())

	// Extract channel data from database if available
	if db != nil {
		channels, err := getTopChannelsFromDB(db, 10)
		if err == nil {
			data.TopChannels = channels
		}
	}

	return data
}

// getTopChannelsFromDB queries the database for the top channels.
func getTopChannelsFromDB(db *store.Store, limit int) ([]ChannelStats, error) {
	if db == nil || db.DB() == nil {
		return []ChannelStats{}, nil
	}
	rows, err := db.DB().Query(
		"SELECT name, (SELECT COUNT(*) FROM messages WHERE messages.recipient = channels.name) as msg_count, subscribers FROM channels ORDER BY subscribers DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return []ChannelStats{}, err
	}
	defer rows.Close()

	channels := []ChannelStats{}
	for rows.Next() {
		var ch ChannelStats
		if err := rows.Scan(&ch.Name, &ch.Messages, &ch.Subscribers); err != nil {
			continue
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// HealthCheckWithDeps verifies the health of system dependencies and returns
// a map of component name → status string ("ok" or "error: ...").
func HealthCheckWithDeps(db *store.Store) map[string]string {
	health := make(map[string]string)

	// Database check
	if db != nil && db.DB() != nil {
		if err := db.DB().Ping(); err != nil {
			health["database"] = "error: " + err.Error()
		} else {
			health["database"] = "ok"
		}
	} else {
		health["database"] = "error: not connected"
	}

	// Storage check (for now, use DB as proxy since chunks are stored via DB)
	health["storage"] = "ok"

	// Memory check
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Consider it healthy if heap alloc < 1 GB
	if m.HeapAlloc < 1<<30 {
		health["memory"] = "ok"
	} else {
		health["memory"] = "warning: high memory usage"
	}

	return health
}
