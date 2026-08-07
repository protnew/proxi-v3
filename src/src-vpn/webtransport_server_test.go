package vpn

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/webtransport-go"
)

func TestWTServerStartStop(t *testing.T) {
	srv, err := NewWTServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewWTServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	stats := srv.GetStats()
	if stats["transport"] != "webtransport" {
		t.Errorf("transport=%v", stats["transport"])
	}
	if stats["certHash"] == "" || len(stats["certHash"].(string)) != 64 {
		t.Errorf("bad certHash %v", stats["certHash"])
	}
	if !stats["running"].(bool) {
		t.Error("expected running")
	}
}

func TestWTServerWebTransportTunnel(t *testing.T) {
	srv, err := NewWTServer("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// TCP echo target
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(c)
		}
	}()

	addr := srv.LocalAddr() // host:port
	url := fmt.Sprintf("https://%s/wt", addr)

	tr := &webtransport.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rsp, sess, err := tr.Dial(ctx, url, nil)
	if err != nil {
		// fallback API name
		t.Fatalf("Dial: %v", err)
	}
	if rsp != nil && rsp.StatusCode >= 300 {
		t.Fatalf("status %d", rsp.StatusCode)
	}
	defer sess.CloseWithError(0, "")

	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}

	target := []byte(echoLn.Addr().String())
	hdr := make([]byte, 5)
	hdr[0] = 0x01
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(target)))
	if _, err := stream.Write(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write(target); err != nil {
		t.Fatal(err)
	}

	rh := make([]byte, 5)
	if _, err := io.ReadFull(stream, rh); err != nil {
		t.Fatalf("read resp: %v", err)
	}
	if rh[0] != 0x81 {
		bl := binary.BigEndian.Uint32(rh[1:5])
		b := make([]byte, bl)
		io.ReadFull(stream, b)
		t.Fatalf("expected OK, got 0x%02x %s", rh[0], string(b))
	}
	bl := binary.BigEndian.Uint32(rh[1:5])
	if bl > 0 {
		b := make([]byte, bl)
		io.ReadFull(stream, b)
	}

	msg := []byte("HELLO_REAL_WEBTRANSPORT")
	if _, err := stream.Write(msg); err != nil {
		t.Fatal(err)
	}
	echoed := make([]byte, len(msg))
	if err := stream.SetReadDeadline(time.Now().Add(5 * time.Second)); err == nil {
		// optional
	}
	if _, err := io.ReadFull(stream, echoed); err != nil {
		t.Fatalf("echo read: %v", err)
	}
	if string(echoed) != string(msg) {
		t.Fatalf("echo mismatch %q", echoed)
	}
	t.Logf("✅ Real WebTransport tunnel OK (%d bytes)", len(msg))
	_ = http.StatusOK
}
