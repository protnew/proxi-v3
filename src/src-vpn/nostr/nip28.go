// Package nostr implements the Nostr protocol.
//
// nip28.go implements NIP-28 Public Chat.
// Spec: https://github.com/nostr-protocol/nips/blob/master/28.md
//
// NIP-28 defines public channels using kind 40 (channel creation),
// kind 41 (channel metadata), and kind 42 (channel message).
package nostr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
)

// Channel represents a NIP-28 public chat channel.
type Channel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	About     string `json:"about"`
	Picture   string `json:"picture"`
	CreatedAt int64  `json:"created_at"`
}

// ChannelMetadata is the content JSON for channel creation (kind 40) and metadata (kind 41).
type ChannelMetadata struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Picture string `json:"picture"`
}

// CreateChannelEvent creates a NIP-28 kind=40 channel creation event.
// The content field contains JSON-encoded channel metadata.
// The event ID becomes the channel ID.
//
// Parameters:
//   - name:    channel name
//   - about:   channel description
//   - picture: channel picture URL
//   - privKey: hex-encoded private key of the channel creator
//
// Returns the created Event (kind 40) or an error.
func CreateChannelEvent(name, about, picture string, privKey string) (Event, error) {
	privBytes, err := hex.DecodeString(privKey)
	if err != nil {
		return Event{}, fmt.Errorf("nip28: decode private key: %w", err)
	}
	privKeyObj, _ := btcec.PrivKeyFromBytes(privBytes)
	pubKeyHex := hex.EncodeToString(privKeyObj.PubKey().SerializeCompressed())

	metadata := ChannelMetadata{
		Name:    name,
		About:   about,
		Picture: picture,
	}
	contentBytes, err := json.Marshal(metadata)
	if err != nil {
		return Event{}, fmt.Errorf("nip28: marshal metadata: %w", err)
	}

	evt := Event{
		PubKey:    pubKeyHex,
		CreatedAt: time.Now().Unix(),
		Kind:      40, // Channel creation
		Tags:      [][]string{},
		Content:   string(contentBytes),
	}

	evt.ID = ComputeEventID(&evt)

	// Sign the event
	idBytes, err := hex.DecodeString(evt.ID)
	if err != nil {
		return Event{}, fmt.Errorf("nip28: decode event ID: %w", err)
	}
	sig := ecdsa.Sign(privKeyObj, idBytes)
	evt.Sig = hex.EncodeToString(sig.Serialize())

	return evt, nil
}

// SendChannelMessage creates a NIP-28 kind=42 channel message event.
// The message is tagged with the channel ID using an "e" tag.
//
// Parameters:
//   - channelID: the channel event ID (kind 40 event ID)
//   - text:      message text
//   - privKey:   hex-encoded private key of the sender
//
// Returns the created Event (kind 42) or an error.
func SendChannelMessage(channelID, text string, privKey string) (Event, error) {
	privBytes, err := hex.DecodeString(privKey)
	if err != nil {
		return Event{}, fmt.Errorf("nip28: decode private key: %w", err)
	}
	privKeyObj, _ := btcec.PrivKeyFromBytes(privBytes)
	pubKeyHex := hex.EncodeToString(privKeyObj.PubKey().SerializeCompressed())

	evt := Event{
		PubKey:    pubKeyHex,
		CreatedAt: time.Now().Unix(),
		Kind:      42, // Channel message
		Tags: [][]string{
			{"e", channelID, "", "root"}, // e-tag references the channel
		},
		Content: text,
	}

	evt.ID = ComputeEventID(&evt)

	// Sign the event
	idBytes, err := hex.DecodeString(evt.ID)
	if err != nil {
		return Event{}, fmt.Errorf("nip28: decode event ID: %w", err)
	}
	sig := ecdsa.Sign(privKeyObj, idBytes)
	evt.Sig = hex.EncodeToString(sig.Serialize())

	return evt, nil
}
