# Sunchain

Sunchain is a lightweight Go prototype for a proof-of-history + proof-of-stake inspired L1 node, paired with a static explorer UI that showcases live block feeds, validator health, and staking UX. The node exposes a JSON-RPC API, gossips peer lists over TCP, and produces blocks on a configurable interval. The explorer UI renders a Solana-style dashboard with simulated metrics when a WebSocket stream is unavailable.

## Features

- **Proof of History (PoH) tick generator** to create deterministic hashes and sequences.
- **Proof of Stake (PoS) validator selection** weighted by stake.
- **Block production loop** with configurable intervals.
- **TCP gossip** for peer discovery and heartbeats.
- **JSON-RPC server** for health, validator, and latest block queries.
- **Static explorer UI** for live block feeds, validator health, and staking interactions.

## Repository layout

```
cmd/sunchain/        # Node entrypoint
internal/config/     # Configuration defaults
internal/consensus/  # PoH + PoS logic
internal/gossip/     # TCP gossip networking
internal/node/       # Node orchestration + RPC handler
internal/rpc/        # JSON-RPC server
internal/types/      # Shared types
index.html           # Explorer UI
script.js            # Explorer UI logic
style.css            # Explorer UI styles
```

## Getting started

### Prerequisites

- Go 1.21+

### Run a node

```bash
go run ./cmd/sunchain
```

### Configuration flags

| Flag | Default | Description |
| --- | --- | --- |
| `-node-id` | `node-1` | Unique node identifier |
| `-rpc-addr` | `0.0.0.0:8080` | JSON-RPC listen address |
| `-gossip-addr` | `0.0.0.0:9000` | TCP gossip listen address |
| `-block-interval` | `400ms` | Block production interval |

Example:

```bash
go run ./cmd/sunchain -node-id validator-1 -rpc-addr 0.0.0.0:8081 -gossip-addr 0.0.0.0:9100 -block-interval 600ms
```

## JSON-RPC API

The node exposes a JSON-RPC 2.0 endpoint at `POST /rpc`.

### Health

```bash
curl -s http://localhost:8080/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"getHealth","id":1}'
```

### Validators

```bash
curl -s http://localhost:8080/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"getValidators","id":2}'
```

### Latest block

```bash
curl -s http://localhost:8080/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"getLatestBlock","id":3}'
```

## Gossip protocol

- Peers connect over TCP to the configured `-gossip-addr`.
- Messages are JSON-encoded with `peer_list` and `heartbeat` types.
- The node gossips discovered peers to new connections.

## Explorer UI

Open `index.html` in a browser. The UI connects to a WebSocket at `ws://localhost:8080/blocks` if available; when the connection fails, it falls back to a simulated block stream and synthetic metrics so the dashboard is always active. The staking panel is a front-end mock to demonstrate user flows.

## Notes

- This project is a prototype for experimentation; it does not persist chain data and does not implement consensus finality.
- The WebSocket block stream endpoint is not implemented in the Go node yet; the UI handles this gracefully by switching to simulated data.
