package ipfs

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// Node represents an embedded or managed IPFS daemon.
type Node struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// StartNode attempts to start a local IPFS daemon.
// It checks if IPFS is installed and starts it. If the daemon is already running,
// it just returns successfully.
func StartNode(ctx context.Context) (*Node, error) {
	// First check if already available
	client := NewClient("", "")
	if client.IsAvailable() {
		log.Println("✅ Local IPFS daemon is already running.")
		return &Node{}, nil
	}

	// Check if IPFS CLI is available
	_, err := exec.LookPath("ipfs")
	if err != nil {
		return nil, fmt.Errorf("ipfs executable not found in PATH: %w", err)
	}

	// Ensure repo is initialized (ipfs init)
	initCmd := exec.Command("ipfs", "init")
	_ = initCmd.Run() // Ignore error, it usually means already initialized

	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "ipfs", "daemon")

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start IPFS daemon: %w", err)
	}

	node := &Node{
		cmd:    cmd,
		cancel: cancel,
	}

	// Wait for daemon to become available
	log.Println("⏳ Waiting for IPFS daemon to start...")
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		if client.IsAvailable() {
			log.Println("✅ Local IPFS daemon started successfully.")
			return node, nil
		}
	}

	// If we reach here, it failed to become available in time
	cancel()
	return nil, fmt.Errorf("IPFS daemon did not start within 15 seconds")
}

// Stop stops the managed IPFS daemon.
func (n *Node) Stop() {
	if n.cancel != nil {
		log.Println("🛑 Stopping IPFS daemon...")
		n.cancel()
		if n.cmd != nil {
			_ = n.cmd.Wait()
		}
	}
}
