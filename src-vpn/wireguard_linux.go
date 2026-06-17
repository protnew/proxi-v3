package vpn

import (
	"fmt"
	"log"
	"os/exec"
)

// CreateWireGuardInterface creates a new WireGuard interface using the `ip` command.
// This is typically only available on Linux with root privileges.
func CreateWireGuardInterface(name string) error {
	log.Printf("[VPN] Creating WireGuard interface: %s", name)
	
	cmd := exec.Command("ip", "link", "add", name, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[VPN] Failed to create WireGuard interface %s: %s", name, string(output))
		return fmt.Errorf("ip link add: %w (output: %s)", err, string(output))
	}

	log.Printf("[VPN] WireGuard interface %s created successfully", name)
	return nil
}
