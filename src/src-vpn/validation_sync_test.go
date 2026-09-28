package vpn

import (
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func TestMaxMessageLenSyncWithStore(t *testing.T) {
	if MaxMessageLen != store.MaxMessageLen {
		t.Fatalf("MaxMessageLen drift: vpn=%d store=%d", MaxMessageLen, store.MaxMessageLen)
	}
}
