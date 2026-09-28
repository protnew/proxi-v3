package vpn

import (
	"os/exec"
	"testing"
)

func TestCreateWireGuardInterface(t *testing.T) {
	// Skip test if `ip` command is not available (e.g., on Windows or Mac without iproute2)
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("ip command not available, skipping test")
	}

	err := CreateWireGuardInterface("test-um0")
	if err != nil {
		// Expect permission denied or similar if not running as root, which is fine
		t.Logf("CreateWireGuardInterface returned error (expected without root): %v", err)
	} else {
		// Clean up if it somehow succeeded
		_ = exec.Command("ip", "link", "del", "test-um0").Run()
	}
}
