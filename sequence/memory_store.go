package sequence

import (
	"context"
	"sync"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// MemoryStore implements Store using in-memory storage.
// Suitable for single-instance deployments, development, and testing.
type MemoryStore struct {
	rules collections.ConcurrentMap[string, *Rule]
	locks collections.ConcurrentMap[string, *sync.Mutex]
}

// NewMemoryStore creates a new in-memory sequence store.
func NewMemoryStore() Store {
	return &MemoryStore{
		rules: collections.NewConcurrentHashMap[string, *Rule](),
		locks: collections.NewConcurrentHashMap[string, *sync.Mutex](),
	}
}

// Register preloads rules into the memory store.
// Existing rules with the same key will be overwritten.
func (s *MemoryStore) Register(rules ...*Rule) {
	for _, rule := range rules {
		s.rules.Put(rule.Key, rule)
	}
}

func (s *MemoryStore) Load(_ context.Context, key string) (*Rule, error) {
	rule, ok := s.rules.Get(key)
	if !ok || !rule.IsActive {
		return nil, ErrRuleNotFound
	}

	// Return a copy to prevent external mutation
	copied := *rule

	return &copied, nil
}

func (s *MemoryStore) Increment(_ context.Context, key string, step int, count int, startValue int, resetNeeded bool) (int, error) {
	mu, _ := s.locks.GetOrCompute(key, func() *sync.Mutex { return &sync.Mutex{} })

	mu.Lock()
	defer mu.Unlock()

	rule, ok := s.rules.Get(key)
	if !ok {
		return 0, ErrRuleNotFound
	}

	if resetNeeded {
		rule.CurrentValue = startValue
		now := timex.Now()
		rule.LastResetAt = &now
	}

	rule.CurrentValue += step * count

	return rule.CurrentValue, nil
}
