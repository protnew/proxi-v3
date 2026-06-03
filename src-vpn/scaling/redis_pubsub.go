package scaling

import (
	"context"
	"fmt"
	"sync"

	goredis "github.com/redis/go-redis/v9"
)

// RedisClient defines the interface for Redis operations used by pub/sub.
type RedisClient interface {
	Publish(channel string, data []byte) error
	Subscribe(channel string) (<-chan []byte, error)
	Close() error
}

// RedisPubSub manages Redis-based publish/subscribe for horizontal scaling.
type RedisPubSub struct {
	client  RedisClient
	channel string
}

// NewRedisPubSub creates a new RedisPubSub connected to the given Redis address.
// If the connection fails, the client will be nil and IsAvailable() returns false.
func NewRedisPubSub(addr, password string, db int) *RedisPubSub {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisPubSub{
		client:  &realRedisClient{client: rdb},
		channel: "unkillable-messenger",
	}
}

// NewRedisPubSubWithClient creates a RedisPubSub with a custom client (for testing).
func NewRedisPubSubWithClient(client RedisClient, channel string) *RedisPubSub {
	return &RedisPubSub{
		client:  client,
		channel: channel,
	}
}

// Publish sends a message to the Redis channel.
func (r *RedisPubSub) Publish(msg []byte) error {
	if r.client == nil {
		return fmt.Errorf("redis client not available")
	}
	return r.client.Publish(r.channel, msg)
}

// Subscribe returns a channel that receives messages from the Redis channel.
func (r *RedisPubSub) Subscribe() (<-chan []byte, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}
	return r.client.Subscribe(r.channel)
}

// IsAvailable returns true if the Redis client is configured.
func (r *RedisPubSub) IsAvailable() bool {
	return r.client != nil
}

// Close closes the Redis client connection.
func (r *RedisPubSub) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

// --- Real Redis client implementation ---

type realRedisClient struct {
	client *goredis.Client
}

func (r *realRedisClient) Publish(channel string, data []byte) error {
	return r.client.Publish(context.Background(), channel, data).Err()
}

func (r *realRedisClient) Subscribe(channel string) (<-chan []byte, error) {
	sub := r.client.Subscribe(context.Background(), channel)
	ch := make(chan []byte, 256)

	go func() {
		defer close(ch)
		msgCh := sub.Channel()
		for msg := range msgCh {
			ch <- []byte(msg.Payload)
		}
	}()

	return ch, nil
}

func (r *realRedisClient) Close() error {
	return r.client.Close()
}

// --- Mock Redis client for testing ---

// MockRedisClient is an in-memory mock of RedisClient for testing.
type MockRedisClient struct {
	mu      sync.Mutex
	subs    []chan []byte
	closed  bool
}

// NewMockRedisClient creates a new MockRedisClient.
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{}
}

// Publish sends data to all subscribed channels.
func (m *MockRedisClient) Publish(channel string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("client closed")
	}
	for _, ch := range m.subs {
		select {
		case ch <- data:
		default:
			// Drop message if channel is full
		}
	}
	return nil
}

// Subscribe returns a channel that receives published messages.
func (m *MockRedisClient) Subscribe(channel string) (<-chan []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan []byte, 256)
	m.subs = append(m.subs, ch)
	return ch, nil
}

// Close marks the client as closed and closes all subscriber channels.
func (m *MockRedisClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, ch := range m.subs {
		close(ch)
	}
	m.subs = nil
	return nil
}
