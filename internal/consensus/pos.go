package consensus

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
	"time"

	"sunchain/internal/types"
)

func SelectValidator(validators []types.Validator, seed []byte) (types.Validator, error) {
	if len(validators) == 0 {
		return types.Validator{}, fmt.Errorf("no validators available")
	}

	var total uint64
	for _, v := range validators {
		total += v.Stake
	}
	if total == 0 {
		return types.Validator{}, fmt.Errorf("total stake is zero")
	}

	var seedValue int64
	if len(seed) >= 8 {
		seedValue = int64(binary.LittleEndian.Uint64(seed[:8]))
	} else {
		var fallback [8]byte
		if _, err := rand.Read(fallback[:]); err != nil {
			seedValue = time.Now().UnixNano()
		} else {
			seedValue = int64(binary.LittleEndian.Uint64(fallback[:]))
		}
	}

	rng := mathrand.New(mathrand.NewSource(seedValue))
	target := rng.Uint64() % total

	var cumulative uint64
	for _, v := range validators {
		cumulative += v.Stake
		if target < cumulative {
			return v, nil
		}
	}

	return validators[len(validators)-1], nil
}
