package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
)

// httpBindAddr is the TCP address the messenger HTTP server listens on.
// Must match http.Server.Addr in startup.go. Loopback-only: LAN IPs in this
// JSON are inventory, not a reachable bind.
func httpBindAddr(port string) string {
	if port == "" {
		port = "8090"
	}
	return "127.0.0.1:" + port
}

func httpBindIsLoopback(bind string) bool {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}

// GET /api/network/lan — VPN-LAN-001
// Returns private IPv4 addresses of this host so phone can open UI on same Wi‑Fi.
func handleLANInfo(w http.ResponseWriter, r *http.Request) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	bind := httpBindAddr(port)
	loopback := httpBindIsLoopback(bind)
	var ips []string
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				ip = ip.To4()
				if ip == nil {
					continue
				}
				// private only
				if ip[0] == 10 || (ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) || (ip[0] == 192 && ip[1] == 168) {
					ips = append(ips, ip.String())
				}
			}
		}
	}
	urls := make([]string, 0, len(ips))
	if !loopback {
		for _, ip := range ips {
			urls = append(urls, "http://"+ip+":"+port+"/?role=bob")
		}
	}
	note := "Bind is loopback. phone_urls empty until the process listens on LAN. Laptop stays ?role=alice on localhost."
	if !loopback {
		note = "Same Wi‑Fi only. Open phone_urls on the phone (role=bob). Laptop stays ?role=alice on localhost."
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                 true,
		"port":               port,
		"lan_ips":            ips,
		"phone_urls":         urls,
		"note":               note,
		"bind":               bind,
		"reachable_from_lan": !loopback,
	})
}
