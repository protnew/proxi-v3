// File: wireguard_rpc.go
// Split from wireguard.go: peer management + RPC handler.

package vpn

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// VPNState — состояние VPN

// Peer — узел в mesh-сети

// Config — конфигурация VPN

// Status — полный статус для UI

// Manager — управление WireGuard VPN

// NewManager создаёт VPN менеджер
func (m *Manager) AddPeer(name, pubKey, endpoint string) error {
	if len(pubKey) < 16 {
		return fmt.Errorf("public key too short (min 16 chars, got %d)", len(pubKey))
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	ip := m.allocatePeerIP()
	peer := &Peer{
		ID:         pubKey[:16],
		Name:       name,
		PublicKey:  pubKey,
		Endpoint:   endpoint,
		AllowedIPs: ip + "/32",
		IsExitNode: false,
		Online:     false,
	}
	m.peers[peer.ID] = peer
	return nil
}

// RemovePeer — удалить пира
func (m *Manager) RemovePeer(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.peers[peerID]; ok {
		m.removePeerFromInterface(p)
		delete(m.peers, peerID)
	}
	return nil
}

// GetPublicKey — вернуть свой публичный ключ (для обмена)
func (m *Manager) GetPublicKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pubKey
}

// ========== Internal methods ==========

func (m *Manager) allocatePeerIP() string {
	i := len(m.peers) + 2 // .1 = мы
	return fmt.Sprintf("10.77.0.%d", i)
}

func (m *Manager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkPeers()
		}
	}
}

func (m *Manager) checkPeers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, p := range m.peers {
		if p.Endpoint != "" {
			// Ping peer endpoint
			conn, err := net.DialTimeout("udp", p.Endpoint, 3*time.Second)
			if err == nil {
				conn.Close()
				p.Online = true
				p.LastSeen = time.Now().Unix()
			} else {
				p.Online = false
			}
			m.peers[id] = p
		}
	}
}

// ========== JSON-RPC interface for Rust IPC ==========

// HandleRPC — обработка JSON-RPC запроса от Tauri/Rust
func (m *Manager) HandleRPC(request []byte) []byte {
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(request, &req); err != nil {
		return rpcError(-32700, "parse error")
	}

	var result interface{}
	var err error

	switch req.Method {
	case "get_status":
		result = m.GetStatus()
	case "get_public_key":
		result = map[string]string{"publicKey": m.GetPublicKey()}
	case "start_exit_node":
		err = m.StartExitNode(context.Background())
		if err == nil {
			result = map[string]string{"status": "sharing"}
		}
	case "stop_exit_node":
		err = m.StopExitNode()
		if err == nil {
			result = map[string]string{"status": "stopped"}
		}
	case "connect_to_exit_node":
		result, err = connectExitViaPipe(req.Params)
	case "start_local_tunnel":
		err = m.StartLocalTunnel()
		if err == nil {
			result = map[string]string{"status": "connected", "mode": "local"}
		}
	case "start_real_tunnel":
		var params struct {
			Listen   string `json:"listen"`
			Upstream string `json:"upstream"`
		}
		_ = json.Unmarshal(req.Params, &params)
		err = m.StartRealTunnel(params.Listen, params.Upstream)
		if err == nil {
			st := m.GetStatus()
			result = map[string]interface{}{
				"status":      "connected",
				"mode":        st.Mode,
				"socksAddr":   st.SocksAddr,
				"transport":   st.Transport,
				"realTraffic": st.RealTraffic,
			}
		}
	case "check_egress_ip":
		var ip string
		ip, err = m.CheckEgressIP()
		if err == nil {
			result = map[string]string{"ip": ip}
		}
	case "disconnect":
		err = m.Disconnect()
		if err == nil {
			result = map[string]string{"status": "disconnected"}
		}
	case "unlock", "disarm":
		result, err = helperVerbViaPipe(req.Method)
	case "add_peer":
		var params struct {
			Name      string `json:"name"`
			PublicKey string `json:"publicKey"`
			Endpoint  string `json:"endpoint"`
		}
		json.Unmarshal(req.Params, &params)
		err = m.AddPeer(params.Name, params.PublicKey, params.Endpoint)
		if err == nil {
			result = map[string]string{"status": "added"}
		}
	case "remove_peer":
		var params struct {
			PeerID string `json:"peerId"`
		}
		json.Unmarshal(req.Params, &params)
		err = m.RemovePeer(params.PeerID)
		if err == nil {
			result = map[string]string{"status": "removed"}
		}
	case "start_egress_listener", "stop_egress", "create_invite", "connect_invite":
		result, err = donorRPC(m, req.Method, req.Params, m.exitAuth())
	default:
		return rpcError(-32601, "method not found: "+req.Method)
	}

	if err != nil {
		return rpcError(-32000, err.Error())
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  result,
	})
	return resp
}

func rpcError(code int, message string) []byte {
	resp, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"error":   map[string]interface{}{"code": code, "message": message},
	})
	return resp
}
