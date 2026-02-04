package config

import "time"

type Config struct {
	NodeID        string
	RPCAddr       string
	GossipAddr    string
	BlockInterval time.Duration
}

func New() Config {
	return Config{
		NodeID:        "node-1",
		RPCAddr:       "0.0.0.0:8080",
		GossipAddr:    "0.0.0.0:9000",
		BlockInterval: 400 * time.Millisecond,
	}
}
