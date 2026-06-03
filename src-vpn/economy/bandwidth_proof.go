package economy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// TransferRecord records a bandwidth transfer between two nodes.
type TransferRecord struct {
	SenderID   string    // node that sent data
	ReceiverID string    // node that received data
	BytesSent  int64     // number of bytes transferred
	Timestamp  time.Time // when the transfer occurred
	Signature  []byte    // signature over the record fields
}

// transferSignPayload creates the byte payload to sign/verify.
func transferSignPayload(tr *TransferRecord) []byte {
	h := sha256.New()
	h.Write([]byte(tr.SenderID))
	h.Write([]byte(tr.ReceiverID))
	// encode BytesSent as hex to avoid ambiguity
	h.Write([]byte(hex.EncodeToString([]byte{
		byte(tr.BytesSent >> 56),
		byte(tr.BytesSent >> 48),
		byte(tr.BytesSent >> 40),
		byte(tr.BytesSent >> 32),
		byte(tr.BytesSent >> 24),
		byte(tr.BytesSent >> 16),
		byte(tr.BytesSent >> 8),
		byte(tr.BytesSent),
	})))
	h.Write([]byte(tr.Timestamp.Format(time.RFC3339Nano)))
	return h.Sum(nil)
}

// RecordTransfer creates a TransferRecord signed by the sender's private key.
func RecordTransfer(senderID, receiverID string, bytesSent int64, senderKey *ecdsa.PrivateKey) (*TransferRecord, error) {
	tr := &TransferRecord{
		SenderID:   senderID,
		ReceiverID: receiverID,
		BytesSent:  bytesSent,
		Timestamp:  time.Now(),
	}

	payload := transferSignPayload(tr)
	sig, err := ecdsa.SignASN1(rand.Reader, senderKey, payload)
	if err != nil {
		return nil, err
	}
	tr.Signature = sig
	return tr, nil
}

// VerifyTransfer verifies the signature on a TransferRecord using the sender's public key.
func VerifyTransfer(tr *TransferRecord, senderPubKey *ecdsa.PublicKey) bool {
	payload := transferSignPayload(tr)
	return ecdsa.VerifyASN1(senderPubKey, payload, tr.Signature)
}
