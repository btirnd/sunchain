package types

import "time"

type Validator struct {
	ID    string
	Stake uint64
}

type Block struct {
	Height      uint64
	ProducerID  string
	Hash        string
	PrevHash    string
	Timestamp   time.Time
	PoHSequence uint64
}
