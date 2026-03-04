package sequence

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func ptr[T any](v T) *T { return &v }

func insertTestRule(t *testing.T, db orm.DB, rule *RuleModel) {
	t.Helper()

	if rule.ID == "" {
		rule.ID = id.Generate()
	}

	wantActive := rule.IsActive
	rule.IsActive = true // Bun skips zero-value bool with default tag, insert as true first

	_, err := db.NewInsert().Model(rule).Exec(context.Background())
	require.NoError(t, err)

	// If the intended is_active is false, update it explicitly after insert
	if !wantActive {
		_, err = db.NewUpdate().
			Model((*RuleModel)(nil)).
			Set("is_active", false).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", rule.ID)
			}).
			Exec(context.Background())
		require.NoError(t, err)
	}
}

func TestDBStore(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store := NewDBStore(env.DB).(*DBStore)

		// Auto-create table
		err := store.Init(env.Ctx)
		require.NoError(t, err, "Init should create table without error")

		t.Run("Load", func(t *testing.T) {
			t.Run("ExistingActiveRule", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "load-active",
					Name:             "Load Active",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "N",
					IsActive:         true,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "load-active")
				}).Exec(env.Ctx) //nolint:errcheck

				rule, err := store.Load(env.Ctx, "load-active")

				require.NoError(t, err)
				assert.Equal(t, "load-active", rule.Key)
				assert.Equal(t, "Load Active", rule.Name)
				assert.Equal(t, 4, rule.SeqLength)
				assert.True(t, rule.IsActive)
			})

			t.Run("NonExistentKey", func(t *testing.T) {
				_, err := store.Load(env.Ctx, "non-existent-db")

				assert.ErrorIs(t, err, ErrRuleNotFound)
			})

			t.Run("InactiveRule", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "load-inactive",
					Name:             "Inactive Rule",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "N",
					IsActive:         false,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "load-inactive")
				}).Exec(env.Ctx) //nolint:errcheck

				_, err := store.Load(env.Ctx, "load-inactive")

				assert.ErrorIs(t, err, ErrRuleNotFound)
			})

			t.Run("RuleWithAllFields", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "load-full",
					Name:             "Full Rule",
					Prefix:           ptr("PRE-"),
					Suffix:           ptr("-SUF"),
					DateFormat:       ptr("yyyyMMdd"),
					SeqLength:        6,
					SeqStep:          2,
					StartValue:       100,
					MaxValue:         9999,
					OverflowStrategy: "reset",
					ResetCycle:       "D",
					CurrentValue:     500,
					IsActive:         true,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "load-full")
				}).Exec(env.Ctx) //nolint:errcheck

				rule, err := store.Load(env.Ctx, "load-full")

				require.NoError(t, err)
				assert.Equal(t, "PRE-", rule.Prefix)
				assert.Equal(t, "-SUF", rule.Suffix)
				assert.Equal(t, "yyyyMMdd", rule.DateFormat)
				assert.Equal(t, 6, rule.SeqLength)
				assert.Equal(t, 2, rule.SeqStep)
				assert.Equal(t, 100, rule.StartValue)
				assert.Equal(t, 9999, rule.MaxValue)
				assert.Equal(t, OverflowReset, rule.OverflowStrategy)
				assert.Equal(t, ResetDaily, rule.ResetCycle)
				assert.Equal(t, 500, rule.CurrentValue)
			})
		})

		t.Run("Increment", func(t *testing.T) {
			t.Run("BasicIncrement", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "inc-basic",
					Name:             "Inc Basic",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "N",
					IsActive:         true,
					CurrentValue:     0,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "inc-basic")
				}).Exec(env.Ctx) //nolint:errcheck

				newVal, err := store.Increment(env.Ctx, "inc-basic", 1, 1, 0, false)

				require.NoError(t, err)
				assert.Equal(t, 1, newVal)
			})

			t.Run("IncrementBatch", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "inc-batch",
					Name:             "Inc Batch",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "N",
					IsActive:         true,
					CurrentValue:     0,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "inc-batch")
				}).Exec(env.Ctx) //nolint:errcheck

				newVal, err := store.Increment(env.Ctx, "inc-batch", 1, 5, 0, false)

				require.NoError(t, err)
				assert.Equal(t, 5, newVal)
			})

			t.Run("ConsecutiveIncrements", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "inc-consec",
					Name:             "Inc Consecutive",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "N",
					IsActive:         true,
					CurrentValue:     0,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "inc-consec")
				}).Exec(env.Ctx) //nolint:errcheck

				val1, err := store.Increment(env.Ctx, "inc-consec", 1, 1, 0, false)
				require.NoError(t, err)
				assert.Equal(t, 1, val1)

				val2, err := store.Increment(env.Ctx, "inc-consec", 1, 1, 0, false)
				require.NoError(t, err)
				assert.Equal(t, 2, val2)
			})

			t.Run("NonExistentKey", func(t *testing.T) {
				_, err := store.Increment(env.Ctx, "inc-non-existent", 1, 1, 0, false)

				assert.ErrorIs(t, err, ErrRuleNotFound)
			})

			t.Run("ResetAndIncrement", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "inc-reset",
					Name:             "Inc Reset",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "D",
					IsActive:         true,
					CurrentValue:     100,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "inc-reset")
				}).Exec(env.Ctx) //nolint:errcheck

				newVal, err := store.Increment(env.Ctx, "inc-reset", 1, 1, 0, true)

				require.NoError(t, err)
				assert.Equal(t, 1, newVal)
			})

			t.Run("ResetWithStartValue", func(t *testing.T) {
				insertTestRule(t, env.DB, &RuleModel{
					Key:              "inc-reset-start",
					Name:             "Inc Reset Start",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: "error",
					ResetCycle:       "D",
					IsActive:         true,
					CurrentValue:     100,
				})
				defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("key", "inc-reset-start")
				}).Exec(env.Ctx) //nolint:errcheck

				newVal, err := store.Increment(env.Ctx, "inc-reset-start", 1, 1, 1000, true)

				require.NoError(t, err)
				assert.Equal(t, 1001, newVal)
			})
		})

		t.Run("ConcurrentIncrements", func(t *testing.T) {
			insertTestRule(t, env.DB, &RuleModel{
				Key:              "inc-concurrent",
				Name:             "Inc Concurrent",
				SeqLength:        6,
				SeqStep:          1,
				OverflowStrategy: "error",
				ResetCycle:       "N",
				IsActive:         true,
				CurrentValue:     0,
			})
			defer env.DB.NewDelete().Model((*RuleModel)(nil)).Where(func(cb orm.ConditionBuilder) {
				cb.Equals("key", "inc-concurrent")
			}).Exec(env.Ctx) //nolint:errcheck

			numGoroutines := 50
			var wg sync.WaitGroup

			for range numGoroutines {
				wg.Go(func() {
					_, err := store.Increment(env.Ctx, "inc-concurrent", 1, 1, 0, false)
					assert.NoError(t, err)
				})
			}
			wg.Wait()

			// Verify final value
			rule, err := store.Load(env.Ctx, "inc-concurrent")
			require.NoError(t, err)
			assert.Equal(t, numGoroutines, rule.CurrentValue)
		})

		t.Run("AutoMigrateIdempotent", func(t *testing.T) {
			// Calling Init again should not fail
			err := store.Init(env.Ctx)
			assert.NoError(t, err, "Second Init should be idempotent")
		})
	})
}
