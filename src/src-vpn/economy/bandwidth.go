package economy

import (
	"fmt"
	"time"
)

// ProofOfBandwidth tracks relayed bytes for bandwidth proofs.
type ProofOfBandwidth struct{}

type BandwidthRecord struct {
	PeerID    string
	BytesIn   int64
	BytesOut  int64
	Timestamp int64
}

func (p *ProofOfBandwidth) Record(peerID string, bytesIn, bytesOut int64) BandwidthRecord {
	return BandwidthRecord{
		PeerID: peerID, BytesIn: bytesIn, BytesOut: bytesOut,
		Timestamp: time.Now().Unix(),
	}
}

// Verify checks bandwidth claims against relay logs.
func (p *ProofOfBandwidth) Verify(record BandwidthRecord, relayLogs []BandwidthRecord) bool {
	totalIn := int64(0)
	for _, log := range relayLogs {
		if log.PeerID == record.PeerID {
			totalIn += log.BytesIn + log.BytesOut
		}
	}
	return totalIn >= record.BytesIn+record.BytesOut
}

func (r BandwidthRecord) String() string {
	return fmt.Sprintf("bw[%s:in=%d,out=%d]", r.PeerID, r.BytesIn, r.BytesOut)
}
