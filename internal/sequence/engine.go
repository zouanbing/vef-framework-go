package sequence

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/sequence"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Engine implements sequence.Generator using a pluggable Store backend.
type Engine struct {
	store sequence.Store
}

// NewGenerator creates a new sequence generator.
func NewGenerator(store sequence.Store) sequence.Generator {
	return &Engine{store: store}
}

func (e *Engine) Generate(ctx context.Context, key string) (string, error) {
	results, err := e.GenerateN(ctx, key, 1)
	if err != nil {
		return "", err
	}

	return results[0], nil
}

func (e *Engine) GenerateN(ctx context.Context, key string, count int) ([]string, error) {
	if count < 1 {
		return nil, sequence.ErrInvalidCount
	}

	rule, err := e.store.Load(ctx, key)
	if err != nil {
		return nil, err
	}

	now := timex.Now()
	resetNeeded := needsReset(rule, now)

	newValue, err := e.store.Increment(ctx, key, rule.SeqStep, count, rule.StartValue, resetNeeded)
	if err != nil {
		return nil, err
	}

	// Handle overflow
	if rule.MaxValue > 0 && newValue > rule.MaxValue {
		switch rule.OverflowStrategy {
		case sequence.OverflowError:
			return nil, sequence.ErrSequenceOverflow
		case sequence.OverflowReset:
			// Reset and re-increment
			newValue, err = e.store.Increment(ctx, key, rule.SeqStep, count, rule.StartValue, true)
			if err != nil {
				return nil, err
			}
		case sequence.OverflowExtend:
			// Allow overflow, just continue with the larger value
		default:
			return nil, sequence.ErrSequenceOverflow
		}
	}

	return buildSerialNumbers(rule, newValue, count, now), nil
}

// needsReset checks if the sequence counter should be reset based on the reset cycle.
func needsReset(rule *sequence.Rule, now timex.DateTime) bool {
	if rule.ResetCycle == sequence.ResetNone {
		return false
	}

	if rule.LastResetAt == nil {
		return false
	}

	last := *rule.LastResetAt

	switch rule.ResetCycle {
	case sequence.ResetDaily:
		return last.BeginOfDay() != now.BeginOfDay()
	case sequence.ResetWeekly:
		return last.BeginOfWeek() != now.BeginOfWeek()
	case sequence.ResetMonthly:
		return last.BeginOfMonth() != now.BeginOfMonth()
	case sequence.ResetQuarterly:
		return last.BeginOfQuarter() != now.BeginOfQuarter()
	case sequence.ResetYearly:
		return last.Year() != now.Year()
	default:
		return false
	}
}

// buildSerialNumbers constructs serial number strings for a batch.
// newValue is the final counter value after incrementing; we work backwards to get each value.
func buildSerialNumbers(rule *sequence.Rule, newValue int, count int, now timex.DateTime) []string {
	results := make([]string, count)
	datePart := sequence.FormatDate(now, rule.DateFormat)

	for i := range count {
		// Values in the batch: newValue - (count-1-i)*step
		seqValue := newValue - (count-1-i)*rule.SeqStep
		results[i] = buildSingleSerialNo(rule, seqValue, datePart)
	}

	return results
}

// buildSingleSerialNo constructs a single serial number string.
func buildSingleSerialNo(rule *sequence.Rule, seqValue int, datePart string) string {
	var sb strings.Builder

	if rule.Prefix != "" {
		sb.WriteString(rule.Prefix)
	}

	if datePart != "" {
		sb.WriteString(datePart)
	}

	_, _ = fmt.Fprintf(&sb, "%0*d", rule.SeqLength, seqValue)

	if rule.Suffix != "" {
		sb.WriteString(rule.Suffix)
	}

	return sb.String()
}
