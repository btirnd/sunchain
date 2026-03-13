package gossip

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

type testConn struct {
	io.Reader
	remote net.Addr
}

func (c *testConn) Read(p []byte) (int, error)       { return c.Reader.Read(p) }
func (c *testConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *testConn) Close() error                     { return nil }
func (c *testConn) LocalAddr() net.Addr              { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9000} }
func (c *testConn) RemoteAddr() net.Addr             { return c.remote }
func (c *testConn) SetDeadline(time.Time) error      { return nil }
func (c *testConn) SetReadDeadline(time.Time) error  { return nil }
func (c *testConn) SetWriteDeadline(time.Time) error { return nil }

func newTestGossip(addr string) *Gossip {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(addr, "node-a", logger)
}

func TestMergePeersNormalizesAndDeduplicates(t *testing.T) {
	g := newTestGossip("127.0.0.1:7000")

	g.AddPeer(" 127.0.0.1:8000 ")
	g.mergePeers([]string{"127.0.0.1:8000", "", "127.0.0.1:7000", "bad"})

	peers := g.Peers()
	if len(peers) != 1 || peers[0] != "127.0.0.1:8000" {
		t.Fatalf("unexpected peers: %#v", peers)
	}
}

func TestHandleConnRegistersInboundRemote(t *testing.T) {
	g := newTestGossip("127.0.0.1:7000")
	conn := &testConn{
		Reader: bytes.NewBufferString("{"),
		remote: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8001},
	}

	g.handleConn(context.Background(), conn)

	peers := g.Peers()
	if len(peers) != 1 || peers[0] != "127.0.0.1:8001" {
		t.Fatalf("expected inbound remote to be registered, got %#v", peers)
	}
}
