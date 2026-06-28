// Package idgen provides a simple single-machine Snowflake-style ID generator.
// Layout (64 bits):
//
//	[63]     sign bit  — always 0
//	[62..22] 41 bits   — milliseconds since custom epoch (gives ~69 years)
//	[21..12] 10 bits   — machine ID (hard-coded to 1 for single-machine use)
//	[11..0]  12 bits   — sequence counter within the same millisecond
package idgen

import (
	"sync"
	"time"
)

const (
	// epoch is the custom start epoch: 2024-01-01 00:00:00 UTC in Unix milliseconds.
	epoch int64 = 1704067200000

	machineBits   = 10
	sequenceBits  = 12
	maxSequence   = (1 << sequenceBits) - 1 // 4095
	machineShift  = sequenceBits
	timestampShift = machineBits + sequenceBits

	// defaultMachineID is the machine identifier for single-node deployments.
	defaultMachineID int64 = 1
)

// Snowflake is a thread-safe single-machine Snowflake ID generator.
type Snowflake struct {
	mu        sync.Mutex
	machineID int64
	sequence  int64
	lastMS    int64
}

// defaultGenerator is the package-level singleton used by NextID().
var defaultGenerator = &Snowflake{machineID: defaultMachineID}

// NextID generates the next unique uint64 ID using the package-level generator.
func NextID() uint64 {
	return defaultGenerator.Next()
}

// Next generates the next unique uint64 ID from this Snowflake generator.
func (s *Snowflake) Next() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMS := time.Now().UnixMilli() - epoch
	if nowMS < 0 {
		nowMS = 0
	}

	if nowMS == s.lastMS {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// Sequence exhausted; spin until the next millisecond.
			for nowMS <= s.lastMS {
				nowMS = time.Now().UnixMilli() - epoch
			}
		}
	} else {
		s.sequence = 0
	}

	s.lastMS = nowMS

	id := (nowMS << timestampShift) | (s.machineID << machineShift) | s.sequence
	return uint64(id)
}
