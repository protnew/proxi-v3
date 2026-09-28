package nostr

import (
    "context"
    "fmt"
)

// Client — Nostr клиент для чата и discovery
type Client struct {
    relays []string
    privKey string
}

// NewClient — создаёт Nostr клиент
func NewClient(relays []string, privKey string) *Client {
    return &Client{
        relays:  relays,
        privKey: privKey,
    }
}

// SendMessage — отправить зашифрованное сообщение (NIP-04)
func (c *Client) SendMessage(ctx context.Context, recipientPubKey, message string) error {
    // TODO: реализация через nostr-tools
    fmt.Printf("Sending to %s: %s\n", recipientPubKey, message)
    return nil
}

// Subscribe — подписка на сообщения от контактов
func (c *Client) Subscribe(ctx context.Context, filter map[string]interface{}) (<-chan Event, error) {
    // TODO: подписка через WebSocket relay
    ch := make(chan Event, 100)
    return ch, nil
}

// Event — Nostr событие
type Event struct {
    ID        string `json:"id"`
    PubKey    string `json:"pubkey"`
    Content   string `json:"content"`
    CreatedAt int64  `json:"created_at"`
    Kind      int    `json:"kind"`
}
