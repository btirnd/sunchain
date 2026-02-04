package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type PoH struct {
	mu        sync.Mutex
	lastHash  []byte
	sequence  uint64
	lastStamp time.Time
}

func NewPoH(seed string) *PoH {
	hash := sha256.Sum256([]byte(seed))
	return &PoH{
		lastHash:  hash[:],
		sequence:  0,
		lastStamp: time.Now().UTC(),
	}
}

func (p *PoH) Tick(data string) (hash string, sequence uint64, timestamp time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	input := fmt.Sprintf("%x:%d:%s:%d", p.lastHash, p.sequence, data, time.Now().UnixNano())
	sum := sha256.Sum256([]byte(input))
	p.lastHash = sum[:]
	p.sequence++
	p.lastStamp = time.Now().UTC()

	return hex.EncodeToString(p.lastHash), p.sequence, p.lastStamp
}

func (p *PoH) State() (hash string, sequence uint64, timestamp time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return hex.EncodeToString(p.lastHash), p.sequence, p.lastStamp
}
