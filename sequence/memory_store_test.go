package sequence

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestNewMemoryStore(t *testing.T) {
	t.Run("CreatesValidStore", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)

		assert.NotNil(t, store, "Store should not be nil")
	})

	t.Run("ImplementsStoreInterface", func(t *testing.T) {
		var _ Store = NewMemoryStore()
	})
}

func TestMemoryStoreRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("SingleRule", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true})

		rule, err := store.Load(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "order", rule.Key)
		assert.Equal(t, "Order No", rule.Name)
	})

	t.Run("MultipleRules", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(
			&Rule{Key: "order", Name: "Order No", IsActive: true},
			&Rule{Key: "invoice", Name: "Invoice No", IsActive: true},
		)

		r1, err := store.Load(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "order", r1.Key)

		r2, err := store.Load(ctx, "invoice")
		require.NoError(t, err)
		assert.Equal(t, "invoice", r2.Key)
	})

	t.Run("OverwriteExistingRule", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Old Name", IsActive: true})
		store.Register(&Rule{Key: "order", Name: "New Name", IsActive: true})

		rule, err := store.Load(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "New Name", rule.Name)
	})
}

func TestMemoryStoreLoad(t *testing.T) {
	ctx := context.Background()

	t.Run("ExistingActiveRule", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, SeqStep: 1})

		rule, err := store.Load(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "order", rule.Key)
		assert.Equal(t, 1, rule.SeqStep)
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)

		_, err := store.Load(ctx, "non-existent")

		assert.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("InactiveRule", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: false})

		_, err := store.Load(ctx, "order")

		assert.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("ReturnsCopy", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 10})

		rule, err := store.Load(ctx, "order")
		require.NoError(t, err)

		// Mutate the copy
		rule.CurrentValue = 999

		// Original should remain unchanged
		original, err := store.Load(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, 10, original.CurrentValue)
	})
}

func TestMemoryStoreIncrement(t *testing.T) {
	ctx := context.Background()

	t.Run("BasicIncrement", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0, SeqStep: 1})

		newVal, err := store.Increment(ctx, "order", 1, 1, 0, false)

		require.NoError(t, err)
		assert.Equal(t, 1, newVal)
	})

	t.Run("IncrementByStep", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0})

		newVal, err := store.Increment(ctx, "order", 5, 1, 0, false)

		require.NoError(t, err)
		assert.Equal(t, 5, newVal)
	})

	t.Run("IncrementBatch", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0})

		newVal, err := store.Increment(ctx, "order", 1, 3, 0, false)

		require.NoError(t, err)
		assert.Equal(t, 3, newVal)
	})

	t.Run("IncrementFromExistingValue", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100})

		newVal, err := store.Increment(ctx, "order", 1, 1, 0, false)

		require.NoError(t, err)
		assert.Equal(t, 101, newVal)
	})

	t.Run("ConsecutiveIncrements", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0})

		val1, err := store.Increment(ctx, "order", 1, 1, 0, false)
		require.NoError(t, err)
		assert.Equal(t, 1, val1)

		val2, err := store.Increment(ctx, "order", 1, 1, 0, false)
		require.NoError(t, err)
		assert.Equal(t, 2, val2)

		val3, err := store.Increment(ctx, "order", 1, 1, 0, false)
		require.NoError(t, err)
		assert.Equal(t, 3, val3)
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)

		_, err := store.Increment(ctx, "non-existent", 1, 1, 0, false)

		assert.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("ResetAndIncrement", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100})

		newVal, err := store.Increment(ctx, "order", 1, 1, 0, true)

		require.NoError(t, err)
		assert.Equal(t, 1, newVal) // reset to 0, then +1
	})

	t.Run("ResetWithStartValue", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100})

		newVal, err := store.Increment(ctx, "order", 1, 1, 1000, true)

		require.NoError(t, err)
		assert.Equal(t, 1001, newVal) // reset to 1000, then +1
	})

	t.Run("ResetSetsLastResetAt", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100})

		_, err := store.Increment(ctx, "order", 1, 1, 0, true)
		require.NoError(t, err)

		// Load the rule to check LastResetAt was set
		rule, _ := store.rules.Get("order")
		assert.NotNil(t, rule.LastResetAt)
	})

	t.Run("NoResetKeepsLastResetAt", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		now := timex.Now()
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0, LastResetAt: &now})

		_, err := store.Increment(ctx, "order", 1, 1, 0, false)
		require.NoError(t, err)

		rule, _ := store.rules.Get("order")
		assert.Equal(t, &now, rule.LastResetAt)
	})
}

func TestMemoryStoreConcurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("ConcurrentIncrements", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0})

		numGoroutines := 100
		step := 1

		var wg sync.WaitGroup
		for range numGoroutines {
			wg.Go(func() {
				_, err := store.Increment(ctx, "order", step, 1, 0, false)
				assert.NoError(t, err)
			})
		}
		wg.Wait()

		rule, _ := store.rules.Get("order")
		assert.Equal(t, numGoroutines*step, rule.CurrentValue)
	})

	t.Run("ConcurrentDifferentKeys", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(
			&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0},
			&Rule{Key: "invoice", Name: "Invoice No", IsActive: true, CurrentValue: 0},
		)

		var wg sync.WaitGroup
		for range 50 {
			wg.Go(func() {
				_, err := store.Increment(ctx, "order", 1, 1, 0, false)
				assert.NoError(t, err)
			})
			wg.Go(func() {
				_, err := store.Increment(ctx, "invoice", 1, 1, 0, false)
				assert.NoError(t, err)
			})
		}
		wg.Wait()

		orderRule, _ := store.rules.Get("order")
		invoiceRule, _ := store.rules.Get("invoice")
		assert.Equal(t, 50, orderRule.CurrentValue)
		assert.Equal(t, 50, invoiceRule.CurrentValue)
	})

	t.Run("ConcurrentLoadAndIncrement", func(t *testing.T) {
		store := NewMemoryStore().(*MemoryStore)
		store.Register(&Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0})

		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				_, _ = store.Load(ctx, "order")
			})
			wg.Go(func() {
				_, _ = store.Increment(ctx, "order", 1, 1, 0, false)
			})
		}
		wg.Wait()

		// Just ensure no panics or data races
	})
}
