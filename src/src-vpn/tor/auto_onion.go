package tor

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// AutoOnionManager manages automatic onion service creation at startup.
type AutoOnionManager struct {
	mu          sync.Mutex
	dialer      *TorDialer
	controlAddr string
	targetPort  int
	dataDir     string
	service     *OnionService
	autoStart   bool
}

// NewAutoOnionManager creates a manager that will start an onion service
// when Tor is available.
func NewAutoOnionManager(controlAddr string, targetPort int, dataDir string) *AutoOnionManager {
	return &AutoOnionManager{
		dialer:      NewTorDialer(""),
		controlAddr: controlAddr,
		targetPort:  targetPort,
		dataDir:     dataDir,
		autoStart:   true,
	}
}

// Start attempts to create an onion service if Tor is running.
func (m *AutoOnionManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dialer.IsTorRunning() {
		log.Printf("[AutoOnion] Tor not running, starting background retry")
		go m.retryLoop()
		return fmt.Errorf("tor not running, will retry in background")
	}

	return m.createService()
}

func (m *AutoOnionManager) createService() error {
	svc, err := CreateOnionService(m.controlAddr, m.targetPort)
	if err != nil {
		return fmt.Errorf("create onion service: %w", err)
	}
	m.service = svc
	log.Printf("[AutoOnion] Onion service created: %s:%d -> localhost:%d",
		svc.OnionAddr, svc.Port, m.targetPort)
	return nil
}

func (m *AutoOnionManager) retryLoop() {
	maxRetries := 10
	interval := 30 * time.Second

	for i := 0; i < maxRetries; i++ {
		time.Sleep(interval)

		m.mu.Lock()
		if m.service != nil {
			m.mu.Unlock()
			return
		}

		if m.dialer.IsTorRunning() {
			log.Printf("[AutoOnion] Tor detected on attempt %d, creating service", i+1)
			if err := m.createService(); err != nil {
				log.Printf("[AutoOnion] Attempt %d failed: %v", i+1, err)
			}
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		log.Printf("[AutoOnion] Attempt %d/%d: Tor not running", i+1, maxRetries)
	}

	log.Printf("[AutoOnion] Max retries reached, giving up")
}

func (m *AutoOnionManager) GetOnionAddress() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.service != nil {
		return m.service.OnionAddr
	}

	addr := GetOnionAddress(m.dataDir)
	if addr != "not-yet-configured.onion" {
		return addr
	}
	return ""
}

func (m *AutoOnionManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.service != nil
}

func (m *AutoOnionManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.service = nil
	m.autoStart = false
	log.Printf("[AutoOnion] Stopped")
}

func (m *AutoOnionManager) SetAutoStart(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoStart = enabled
}

// extractOnionFromResponse parses a Tor control port response to find the onion address.
func extractOnionFromResponse(response string) string {
	idx := strings.Index(response, "ServiceID=")
	if idx < 0 {
		return ""
	}
	start := idx + len("ServiceID=")
	end := start
	for end < len(response) {
		c := response[end]
		if c == '\r' || c == '\n' || c == ' ' || c == '\t' {
			break
		}
		end++
	}
	if end > start {
		return response[start:end]
	}
	return ""
}

// isValidOnionAddress checks if an address looks like a valid .onion address.
func isValidOnionAddress(addr string) bool {
	if len(addr) < 17 {
		return false
	}
	if !strings.HasSuffix(addr, ".onion") {
		return false
	}
	return true
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeHostname(dir, addr string) error {
	return os.WriteFile(dir+"/hostname", []byte(addr), 0644)
}
