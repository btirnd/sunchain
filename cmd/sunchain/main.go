package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sunchain/internal/config"
	"sunchain/internal/logging"
	"sunchain/internal/node"
)

func main() {
	cfg := config.New()
	flag.StringVar(&cfg.NodeID, "node-id", cfg.NodeID, "unique node identifier")
	flag.StringVar(&cfg.RPCAddr, "rpc-addr", cfg.RPCAddr, "RPC listen address")
	flag.StringVar(&cfg.GossipAddr, "gossip-addr", cfg.GossipAddr, "gossip listen address")
	flag.DurationVar(&cfg.BlockInterval, "block-interval", cfg.BlockInterval, "block production interval")
	flag.Parse()

	logger := logging.NewLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	chainNode, err := node.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize node", "error", err)
		os.Exit(1)
	}

	if err := chainNode.Start(ctx); err != nil {
		logger.Error("node exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("node shutdown complete")
	time.Sleep(50 * time.Millisecond)
}
