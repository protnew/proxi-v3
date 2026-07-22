// AUDIT: WebRTC DataChannel extensions.
// T4: unreliable UDP, T16: heartbeat, T18: logging.
package stream

import (
	"context"
	"log/slog"
	"time"
)

// T4: DataChannel config for unreliable UDP (ordered=false)
type DataChannelConfig struct {
	Ordered        bool
	MaxRetransmits int  // -1 = unlimited, 0 = unreliable
	Label          string
	Protocol       string
}

func NewVPNDataChannel() DataChannelConfig {
	return DataChannelConfig{
		Ordered:        false,    // T4: Unreliable UDP mode
		MaxRetransmits: 0,       // No retransmits = UDP semantics
		Label:          "vpn-data",
		Protocol:       "udp",
	}
}

// T16: Heartbeat pings through DataChannel
type HeartbeatManager struct {
	interval time.Duration
	timeout  time.Duration
	lastPong time.Time
	logger   *slog.Logger
}

func NewHeartbeatManager(logger *slog.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		interval: 5 * time.Second,
		timeout:  15 * time.Second,
		lastPong: time.Now(),
		logger:   logger,
	}
}

func (h *HeartbeatManager) Start(ctx context.Context, sendPing func() error, onTimeout func()) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sendPing(); err != nil {
				h.logger.Warn("heartbeat send failed", "error", err)
			}
			if time.Since(h.lastPong) > h.timeout {
				h.logger.Warn("heartbeat timeout — peer may be dead")
				onTimeout()
				return
			}
		}
	}
}

func (h *HeartbeatManager) RecordPong() {
	h.lastPong = time.Now()
}

// T18: WebRTC internals logging
type WebRTCStats struct {
	BytesSent      int64
	BytesReceived  int64
	RoundTripTime  float64
	PacketsLost    int
	Jitter         float64
}

func LogWebRTCStats(logger *slog.Logger, stats WebRTCStats) {
	logger.Info("webrtc stats",
		"bytes_sent", stats.BytesSent,
		"bytes_recv", stats.BytesReceived,
		"rtt_ms", stats.RoundTripTime*1000,
		"packets_lost", stats.PacketsLost,
		"jitter_ms", stats.Jitter*1000,
	)
}
