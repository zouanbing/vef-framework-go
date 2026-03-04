package sequence

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/sequence"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// --- helpers ---

func newTestRule(key string, opts ...func(*sequence.Rule)) *sequence.Rule {
	rule := &sequence.Rule{
		Key:              key,
		Name:             "Test Rule",
		SeqLength:        4,
		SeqStep:          1,
		OverflowStrategy: sequence.OverflowError,
		ResetCycle:       sequence.ResetNone,
		IsActive:         true,
	}
	for _, opt := range opts {
		opt(rule)
	}

	return rule
}

func newTestEngine(rules ...*sequence.Rule) sequence.Generator {
	store := sequence.NewMemoryStore().(*sequence.MemoryStore)
	store.Register(rules...)

	return NewGenerator(store)
}

// --- TestGenerate ---

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("BasicGeneration", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0001", result)
	})

	t.Run("ConsecutiveGeneration", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		r1, err := gen.Generate(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "0001", r1)

		r2, err := gen.Generate(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "0002", r2)

		r3, err := gen.Generate(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "0003", r3)
	})

	t.Run("WithPrefix", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.Prefix = "ORD-"
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "ORD-0001", result)
	})

	t.Run("WithSuffix", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.Suffix = "-SH"
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0001-SH", result)
	})

	t.Run("WithPrefixAndSuffix", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.Prefix = "INV-"
			r.Suffix = "-CN"
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "INV-0001-CN", result)
	})

	t.Run("WithDateFormat", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.Prefix = "ORD"
			r.DateFormat = "yyyyMMdd"
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		// Result format: ORD{yyyyMMdd}0001
		assert.Contains(t, result, "ORD")
		assert.Len(t, result, 3+8+4) // prefix(3) + date(8) + seq(4)
	})

	t.Run("EmptyPrefixSuffixDate", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0001", result)
	})

	t.Run("SeqLengthOne", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.SeqLength = 1
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "1", result)
	})

	t.Run("StepGreaterThanOne", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.SeqStep = 5
		}))

		r1, err := gen.Generate(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "0005", r1)

		r2, err := gen.Generate(ctx, "order")
		require.NoError(t, err)
		assert.Equal(t, "0010", r2)
	})

	t.Run("RuleNotFound", func(t *testing.T) {
		gen := newTestEngine()

		_, err := gen.Generate(ctx, "non-existent")

		assert.ErrorIs(t, err, sequence.ErrRuleNotFound)
	})
}

// --- TestGenerateN ---

func TestGenerateN(t *testing.T) {
	ctx := context.Background()

	t.Run("BatchOfThree", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		results, err := gen.GenerateN(ctx, "order", 3)

		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "0001", results[0])
		assert.Equal(t, "0002", results[1])
		assert.Equal(t, "0003", results[2])
	})

	t.Run("BatchOfOne", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		results, err := gen.GenerateN(ctx, "order", 1)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "0001", results[0])
	})

	t.Run("InvalidCountZero", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		_, err := gen.GenerateN(ctx, "order", 0)

		assert.ErrorIs(t, err, sequence.ErrInvalidCount)
	})

	t.Run("InvalidCountNegative", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		_, err := gen.GenerateN(ctx, "order", -1)

		assert.ErrorIs(t, err, sequence.ErrInvalidCount)
	})

	t.Run("BatchWithStep", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.SeqStep = 2
		}))

		results, err := gen.GenerateN(ctx, "order", 3)

		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "0002", results[0])
		assert.Equal(t, "0004", results[1])
		assert.Equal(t, "0006", results[2])
	})

	t.Run("ConsecutiveBatches", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order"))

		batch1, err := gen.GenerateN(ctx, "order", 2)
		require.NoError(t, err)
		assert.Equal(t, "0001", batch1[0])
		assert.Equal(t, "0002", batch1[1])

		batch2, err := gen.GenerateN(ctx, "order", 2)
		require.NoError(t, err)
		assert.Equal(t, "0003", batch2[0])
		assert.Equal(t, "0004", batch2[1])
	})
}

// --- TestNeedsReset ---

func TestNeedsReset(t *testing.T) {
	now := timex.Now()

	t.Run("ResetNone", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetNone, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("NilLastResetAt", func(t *testing.T) {
		rule := &sequence.Rule{ResetCycle: sequence.ResetDaily, LastResetAt: nil}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("DailyResetSameDay", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetDaily, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("DailyResetDifferentDay", func(t *testing.T) {
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &sequence.Rule{ResetCycle: sequence.ResetDaily, LastResetAt: &yesterday}

		assert.True(t, needsReset(rule, now))
	})

	t.Run("WeeklyResetSameWeek", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetWeekly, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("WeeklyResetDifferentWeek", func(t *testing.T) {
		lastWeek := timex.DateTime(time.Time(now).AddDate(0, 0, -7))
		rule := &sequence.Rule{ResetCycle: sequence.ResetWeekly, LastResetAt: &lastWeek}

		assert.True(t, needsReset(rule, now))
	})

	t.Run("MonthlyResetSameMonth", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetMonthly, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("MonthlyResetDifferentMonth", func(t *testing.T) {
		lastMonth := timex.DateTime(time.Time(now).AddDate(0, -1, 0))
		rule := &sequence.Rule{ResetCycle: sequence.ResetMonthly, LastResetAt: &lastMonth}

		assert.True(t, needsReset(rule, now))
	})

	t.Run("QuarterlyResetSameQuarter", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetQuarterly, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("QuarterlyResetDifferentQuarter", func(t *testing.T) {
		lastQuarter := timex.DateTime(time.Time(now).AddDate(0, -3, 0))
		rule := &sequence.Rule{ResetCycle: sequence.ResetQuarterly, LastResetAt: &lastQuarter}

		assert.True(t, needsReset(rule, now))
	})

	t.Run("YearlyResetSameYear", func(t *testing.T) {
		lastReset := now
		rule := &sequence.Rule{ResetCycle: sequence.ResetYearly, LastResetAt: &lastReset}

		assert.False(t, needsReset(rule, now))
	})

	t.Run("YearlyResetDifferentYear", func(t *testing.T) {
		lastYear := timex.DateTime(time.Time(now).AddDate(-1, 0, 0))
		rule := &sequence.Rule{ResetCycle: sequence.ResetYearly, LastResetAt: &lastYear}

		assert.True(t, needsReset(rule, now))
	})
}

// --- TestOverflow ---

func TestOverflow(t *testing.T) {
	ctx := context.Background()

	t.Run("NoMaxValue", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 9999
			r.MaxValue = 0 // unlimited
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "10000", result)
	})

	t.Run("OverflowErrorStrategy", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 9999
			r.MaxValue = 9999
			r.OverflowStrategy = sequence.OverflowError
		}))

		_, err := gen.Generate(ctx, "order")

		assert.ErrorIs(t, err, sequence.ErrSequenceOverflow)
	})

	t.Run("OverflowResetStrategy", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 9999
			r.MaxValue = 9999
			r.OverflowStrategy = sequence.OverflowReset
			r.StartValue = 0
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0001", result) // reset to 0, then +1
	})

	t.Run("OverflowExtendStrategy", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 9999
			r.MaxValue = 9999
			r.OverflowStrategy = sequence.OverflowExtend
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "10000", result)
	})

	t.Run("OverflowResetWithStartValue", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 9999
			r.MaxValue = 9999
			r.OverflowStrategy = sequence.OverflowReset
			r.StartValue = 100
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0101", result) // reset to 100, then +1
	})

	t.Run("WithinMaxValue", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.CurrentValue = 0
			r.MaxValue = 9999
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0001", result)
	})
}

// --- TestStartValue ---

func TestStartValue(t *testing.T) {
	ctx := context.Background()

	t.Run("NonZeroStartValue", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.StartValue = 100
			r.CurrentValue = 100
		}))

		result, err := gen.Generate(ctx, "order")

		require.NoError(t, err)
		assert.Equal(t, "0101", result) // start at 100, first = 101
	})
}

// --- TestConcurrentGenerate ---

func TestConcurrentGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("ConcurrentSameKey", func(t *testing.T) {
		gen := newTestEngine(newTestRule("order", func(r *sequence.Rule) {
			r.SeqLength = 6
		}))

		numGoroutines := 100
		results := make([]string, numGoroutines)
		var wg sync.WaitGroup

		for i := range numGoroutines {
			wg.Go(func() {
				result, err := gen.Generate(ctx, "order")
				require.NoError(t, err)
				results[i] = result
			})
		}
		wg.Wait()

		// All results should be unique
		unique := collections.NewHashSetFrom(results...)
		assert.Equal(t, numGoroutines, unique.Size(), "all generated results should be unique")
	})
}
