package sequence

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var logger = log.Named("sequence")

// Generator provides serial number generation.
type Generator interface {
	// Generate generates a new serial number for the given rule key.
	Generate(ctx context.Context, key string) (string, error)
	// GenerateN generates N serial numbers for the given rule key in a single atomic operation.
	GenerateN(ctx context.Context, key string, count int) ([]string, error)
}

// Store abstracts rule persistence and atomic counter operations.
// Implementations must guarantee atomicity of Increment (read-modify-write must be serialized per key).
type Store interface {
	// Load retrieves an active rule by key. Returns ErrRuleNotFound if not found or inactive.
	Load(ctx context.Context, key string) (*Rule, error)
	// Increment atomically advances the counter for the given key by step * count,
	// resetting to startValue first if resetNeeded is true.
	// Returns the new counter value after increment (i.e., the last value in the batch).
	Increment(ctx context.Context, key string, step int, count int, startValue int, resetNeeded bool) (newValue int, err error)
}
