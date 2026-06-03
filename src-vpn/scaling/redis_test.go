package scaling

import (
	"testing"
	"time"
)

func TestMockPublishSubscribe(t *testing.T) {
	mock := NewMockRedisClient()
	ps := NewRedisPubSubWithClient(mock, "test-channel")

	// Subscribe
	ch, err := ps.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Publish a message
	msg := []byte("hello world")
	if err := ps.Publish(msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Receive
	select {
	case got := <-ch:
		if string(got) != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}

	// Close
	if err := ps.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	mock := NewMockRedisClient()
	ps := NewRedisPubSubWithClient(mock, "test-channel")

	ch1, _ := ps.Subscribe()
	ch2, _ := ps.Subscribe()

	msg := []byte("broadcast")
	_ = ps.Publish(msg)

	// Both should receive
	for i, ch := range []<-chan []byte{ch1, ch2} {
		select {
		case got := <-ch:
			if string(got) != "broadcast" {
				t.Errorf("subscriber %d got %q, want %q", i, got, "broadcast")
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d timed out", i)
		}
	}

	_ = ps.Close()
}

func TestIsAvailable(t *testing.T) {
	mock := NewMockRedisClient()
	ps := NewRedisPubSubWithClient(mock, "test")
	if !ps.IsAvailable() {
		t.Error("expected IsAvailable = true")
	}

	psNil := NewRedisPubSubWithClient(nil, "test")
	if psNil.IsAvailable() {
		t.Error("expected IsAvailable = false with nil client")
	}
}

func TestNilClientErrors(t *testing.T) {
	ps := NewRedisPubSubWithClient(nil, "test")

	if err := ps.Publish([]byte("msg")); err == nil {
		t.Error("expected error when publishing with nil client")
	}

	if _, err := ps.Subscribe(); err == nil {
		t.Error("expected error when subscribing with nil client")
	}
}

func TestCloseIdempotent(t *testing.T) {
	mock := NewMockRedisClient()
	ps := NewRedisPubSubWithClient(mock, "test")

	if err := ps.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close should not panic
	if err := ps.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
