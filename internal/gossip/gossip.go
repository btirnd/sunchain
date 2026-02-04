package gossip

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

type Message struct {
	Type    string   `json:"type"`
	Peers   []string `json:"peers,omitempty"`
	NodeID  string   `json:"node_id,omitempty"`
	Payload string   `json:"payload,omitempty"`
}

type Gossip struct {
	logger *slog.Logger
	addr   string
	nodeID string
	mu     sync.Mutex
	peers  map[string]struct{}
}

func New(addr, nodeID string, logger *slog.Logger) *Gossip {
	return &Gossip{
		logger: logger,
		addr:   addr,
		nodeID: nodeID,
		peers:  make(map[string]struct{}),
	}
}

func (g *Gossip) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", g.addr)
	if err != nil {
		return fmt.Errorf("gossip listen: %w", err)
	}

	g.logger.Info("gossip listening", "addr", g.addr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			g.logger.Warn("gossip accept failed", "error", err)
			continue
		}

		go g.handleConn(ctx, conn)
	}
}

func (g *Gossip) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	decoder := json.NewDecoder(reader)

	var msg Message
	if err := decoder.Decode(&msg); err != nil {
		g.logger.Warn("gossip decode failed", "error", err)
		return
	}

	switch msg.Type {
	case "peer_list":
		g.mergePeers(msg.Peers)
	case "heartbeat":
		g.logger.Debug("heartbeat received", "node", msg.NodeID)
	default:
		g.logger.Warn("unknown gossip message", "type", msg.Type)
	}

	select {
	case <-ctx.Done():
	default:
		_ = g.send(conn.RemoteAddr().String(), Message{
			Type:  "peer_list",
			Peers: g.Peers(),
		})
	}
}

func (g *Gossip) mergePeers(peers []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, peer := range peers {
		if peer == "" || peer == g.addr {
			continue
		}
		g.peers[peer] = struct{}{}
	}
}

func (g *Gossip) AddPeer(peer string) {
	g.mergePeers([]string{peer})
}

func (g *Gossip) Peers() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	peers := make([]string, 0, len(g.peers))
	for peer := range g.peers {
		peers = append(peers, peer)
	}
	return peers
}

func (g *Gossip) Broadcast(ctx context.Context, payload string) {
	msg := Message{
		Type:    "heartbeat",
		NodeID:  g.nodeID,
		Payload: payload,
	}
	for _, peer := range g.Peers() {
		peer := peer
		go func() {
			if err := g.send(peer, msg); err != nil {
				g.logger.Warn("gossip broadcast failed", "peer", peer, "error", err)
			}
		}()
	}

	select {
	case <-ctx.Done():
	case <-time.After(150 * time.Millisecond):
	}
}

func (g *Gossip) send(peer string, msg Message) error {
	conn, err := net.DialTimeout("tcp", peer, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("dial peer %s: %w", peer, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return nil
}
