package monitoring

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metrics collects and exposes application metrics in Prometheus text format.
type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
	latency  map[string][]time.Duration
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
		latency:  make(map[string][]time.Duration),
	}
}

// IncrementCounter increments a named counter by 1.
func (m *Metrics) IncrementCounter(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

// ObserveLatency records a latency observation for a named metric.
func (m *Metrics) ObserveLatency(name string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency[name] = append(m.latency[name], duration)
}

// SetGauge sets a named gauge to the given value.
func (m *Metrics) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// Handler returns an http.Handler that serves metrics in Prometheus text exposition format.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		m.mu.Lock()
		defer m.mu.Unlock()

		var sb strings.Builder

		// Counters
		var counterNames []string
		for k := range m.counters {
			counterNames = append(counterNames, k)
		}
		sort.Strings(counterNames)
		for _, name := range counterNames {
			sb.WriteString(fmt.Sprintf("# HELP %s_total Total count\n", name))
			sb.WriteString(fmt.Sprintf("# TYPE %s_total counter\n", name))
			sb.WriteString(fmt.Sprintf("%s_total %g\n", name, m.counters[name]))
		}

		// Gauges
		var gaugeNames []string
		for k := range m.gauges {
			gaugeNames = append(gaugeNames, k)
		}
		sort.Strings(gaugeNames)
		for _, name := range gaugeNames {
			sb.WriteString(fmt.Sprintf("# HELP %s Current value\n", name))
			sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", name))
			sb.WriteString(fmt.Sprintf("%s %g\n", name, m.gauges[name]))
		}

		// Latency summaries (simple: count, sum, avg)
		var latNames []string
		for k := range m.latency {
			latNames = append(latNames, k)
		}
		sort.Strings(latNames)
		for _, name := range latNames {
			observations := m.latency[name]
			if len(observations) == 0 {
				continue
			}
			var sum time.Duration
			for _, d := range observations {
				sum += d
			}
			avg := sum / time.Duration(len(observations))
			sb.WriteString(fmt.Sprintf("# HELP %s_duration_seconds Latency\n", name))
			sb.WriteString(fmt.Sprintf("# TYPE %s_duration_seconds summary\n", name))
			sb.WriteString(fmt.Sprintf("%s_duration_seconds_count %d\n", name, len(observations)))
			sb.WriteString(fmt.Sprintf("%s_duration_seconds_sum %f\n", name, sum.Seconds()))
			sb.WriteString(fmt.Sprintf("%s_duration_seconds_avg %f\n", name, avg.Seconds()))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	})
}
