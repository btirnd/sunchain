package node

import (
	"context"
	"fmt"
	"log/slog"
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
	chainMu    sync.Mutex
	latest     types.Block
	poh        *consensus.PoH
	gossip     *gossip.Gossip
	rpcServer  *rpc.Server
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
		cfg:        cfg,
		logger:     logger,
		validators: validators,
		poh:        consensus.NewPoH(cfg.NodeID),
	}

	node.gossip = gossip.New(cfg.GossipAddr, cfg.NodeID, logger)
	node.rpcServer = rpc.New(cfg.RPCAddr, node, logger)

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
			if err := n.tryProduceBlock(); err != nil {
				n.logger.Warn("block production failed", "error", err)
			}
		}
	}
}

func (n *Node) tryProduceBlock() error {
	hash, sequence, timestamp := n.poh.Tick("block")
	validator, err := consensus.SelectValidator(n.validators, []byte(hash))
	if err != nil {
		return err
	}

	if validator.ID != n.cfg.NodeID {
		return nil
	}

	n.chainMu.Lock()
	defer n.chainMu.Unlock()

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

	n.logger.Info("block produced", "height", block.Height, "hash", block.Hash)
	n.gossip.Broadcast(context.Background(), block.Hash)
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
	n.chainMu.Lock()
	defer n.chainMu.Unlock()
	return n.latest, nil
}
