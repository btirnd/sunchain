package node

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"sunchain/internal/config"
	"sunchain/internal/consensus"
	"sunchain/internal/gossip"
	"sunchain/internal/rpc"
	"sunchain/internal/types"
)

type Node struct {
	cfg        config.Config
	logger     *slog.Logger
	validators []types.Validator
	chainMu    sync.RWMutex
	latest     types.Block
	poh        *consensus.PoH
	gossip     *gossip.Gossip
	rpcServer  *rpc.Server

	subsMu       sync.RWMutex
	nextSubID    int
	subscribers  map[int]chan types.Block
	chainStateDB string
}

func New(cfg config.Config, logger *slog.Logger) (*Node, error) {
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node id is required")
	}

	validators := []types.Validator{
		{ID: cfg.NodeID, Stake: 100},
		{ID: "validator-2", Stake: 50},
		{ID: "validator-3", Stake: 25},
	}

	node := &Node{
		cfg:          cfg,
		logger:       logger,
		validators:   validators,
		poh:          consensus.NewPoH(cfg.NodeID),
		subscribers:  make(map[int]chan types.Block),
		chainStateDB: filepath.Join(cfg.DataDir, "chain-state.json"),
	}

	latest, err := loadLatestBlock(node.chainStateDB)
	if err != nil {
		return nil, err
	}
	node.latest = latest
	if latest.Height > 0 {
		node.logger.Info("restored chain state", "height", latest.Height, "hash", latest.Hash)
	}

	node.gossip = gossip.New(cfg.GossipAddr, cfg.NodeID, logger)
	node.rpcServer = rpc.New(cfg.RPCAddr, node, cfg.AllowedOrigin, logger)

	return node, nil
}

func (n *Node) Start(ctx context.Context) error {
	errChan := make(chan error, 3)

	go func() {
		if err := n.gossip.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	go func() {
		if err := n.rpcServer.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	go func() {
		if err := n.produceBlocks(ctx); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	}
}

func (n *Node) produceBlocks(ctx context.Context) error {
	ticker := time.NewTicker(n.cfg.BlockInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := n.tryProduceBlock(ctx); err != nil {
				n.logger.Warn("block production failed", "error", err)
			}
		}
	}
}

func (n *Node) tryProduceBlock(ctx context.Context) error {
	hash, sequence, timestamp := n.poh.Tick("block")
	validator, err := consensus.SelectValidator(n.validators, []byte(hash))
	if err != nil {
		return err
	}

	if validator.ID != n.cfg.NodeID {
		return nil
	}

	n.chainMu.Lock()
	nextHeight := n.latest.Height + 1
	block := types.Block{
		Height:      nextHeight,
		ProducerID:  validator.ID,
		Hash:        hash,
		PrevHash:    n.latest.Hash,
		Timestamp:   timestamp,
		PoHSequence: sequence,
	}
	n.latest = block
	n.chainMu.Unlock()

	if err := persistLatestBlock(n.chainStateDB, block); err != nil {
		return err
	}

	n.logger.Info("block produced", "height", block.Height, "hash", block.Hash)
	n.gossip.Broadcast(ctx, block.Hash)
	n.broadcastBlock(block)
	return nil
}

func (n *Node) Health(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return map[string]any{
		"status":  "ok",
		"node_id": n.cfg.NodeID,
	}, nil
}

func (n *Node) Validators(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return n.validators, nil
}

func (n *Node) LatestBlock(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	n.chainMu.RLock()
	defer n.chainMu.RUnlock()
	return n.latest, nil
}

func (n *Node) SubscribeBlocks(buffer int) (int, <-chan types.Block) {
	if buffer < 1 {
		buffer = 1
	}

	n.subsMu.Lock()
	defer n.subsMu.Unlock()
	n.nextSubID++
	id := n.nextSubID
	ch := make(chan types.Block, buffer)
	n.subscribers[id] = ch
	return id, ch
}

func (n *Node) UnsubscribeBlocks(id int) {
	n.subsMu.Lock()
	defer n.subsMu.Unlock()
	ch, ok := n.subscribers[id]
	if !ok {
		return
	}
	delete(n.subscribers, id)
	close(ch)
}

func (n *Node) broadcastBlock(block types.Block) {
	n.subsMu.RLock()
	defer n.subsMu.RUnlock()
	for id, ch := range n.subscribers {
		select {
		case ch <- block:
		default:
			n.logger.Warn("dropping block for slow subscriber", "subscriber_id", id, "height", block.Height)
		}
	}
}
