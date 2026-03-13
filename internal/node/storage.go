package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sunchain/internal/types"
)

type chainState struct {
	Latest types.Block `json:"latest"`
}

func loadLatestBlock(path string) (types.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return types.Block{}, nil
		}
		return types.Block{}, fmt.Errorf("read chain state: %w", err)
	}

	var state chainState
	if err := json.Unmarshal(data, &state); err != nil {
		return types.Block{}, fmt.Errorf("decode chain state: %w", err)
	}
	return state.Latest, nil
}

func persistLatestBlock(path string, block types.Block) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	payload, err := json.MarshalIndent(chainState{Latest: block}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode chain state: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "chain-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(payload); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("replace chain state: %w", err)
	}
	return nil
}
