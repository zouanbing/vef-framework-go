package sequence

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/timex"
)

const redisSequencePrefix = "vef:sequence:"

// luaIncrement atomically increments the sequence counter.
// KEYS[1] = hash key
// ARGV[1] = step * count
// ARGV[2] = resetNeeded (0 or 1)
// ARGV[3] = startValue
// ARGV[4] = current timestamp (for last_reset_at)
// Returns: new current_value
var luaIncrement = redis.NewScript(`
local key = KEYS[1]
local delta = tonumber(ARGV[1])
local resetNeeded = tonumber(ARGV[2])
local startValue = tonumber(ARGV[3])
local timestamp = ARGV[4]

if resetNeeded == 1 then
    redis.call('HSET', key, 'current_value', startValue, 'last_reset_at', timestamp)
end

local newVal = redis.call('HINCRBY', key, 'current_value', delta)
return newVal
`)

// RedisStore implements Store using Redis for distributed deployments.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-backed sequence store.
func NewRedisStore(client *redis.Client) Store {
	return &RedisStore{client: client}
}

func (s *RedisStore) Load(ctx context.Context, key string) (*Rule, error) {
	rKey := redisSequencePrefix + key

	result, err := s.client.HGetAll(ctx, rKey).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, ErrRuleNotFound
	}

	isActive, _ := strconv.ParseBool(result["is_active"])
	if !isActive {
		return nil, ErrRuleNotFound
	}

	rule := &Rule{
		Key:              result["key"],
		Name:             result["name"],
		Prefix:           result["prefix"],
		Suffix:           result["suffix"],
		DateFormat:       result["date_format"],
		OverflowStrategy: OverflowStrategy(result["overflow_strategy"]),
		ResetCycle:       ResetCycle(result["reset_cycle"]),
		IsActive:         isActive,
	}

	rule.SeqLength, _ = strconv.Atoi(result["seq_length"])
	rule.SeqStep, _ = strconv.Atoi(result["seq_step"])
	rule.StartValue, _ = strconv.Atoi(result["start_value"])
	rule.MaxValue, _ = strconv.Atoi(result["max_value"])
	rule.CurrentValue, _ = strconv.Atoi(result["current_value"])

	if lastReset := result["last_reset_at"]; lastReset != "" {
		if dt, err := timex.Parse(lastReset); err == nil {
			rule.LastResetAt = &dt
		}
	}

	return rule, nil
}

func (s *RedisStore) Increment(ctx context.Context, key string, step int, count int, startValue int, resetNeeded bool) (int, error) {
	rKey := redisSequencePrefix + key

	exists, err := s.client.Exists(ctx, rKey).Result()
	if err != nil {
		return 0, err
	}

	if exists == 0 {
		return 0, ErrRuleNotFound
	}

	resetFlag := 0
	if resetNeeded {
		resetFlag = 1
	}

	now := timex.Now().Format(time.DateTime)

	result, err := luaIncrement.Run(ctx, s.client, []string{rKey},
		step*count,
		resetFlag,
		startValue,
		now,
	).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrRuleNotFound
		}

		return 0, err
	}

	return result, nil
}

// RegisterRule stores a rule in Redis as a hash.
// This is a helper for setting up rules in Redis.
func (s *RedisStore) RegisterRule(ctx context.Context, rule *Rule) error {
	rKey := redisSequencePrefix + rule.Key

	fields := map[string]any{
		"key":               rule.Key,
		"name":              rule.Name,
		"prefix":            rule.Prefix,
		"suffix":            rule.Suffix,
		"date_format":       rule.DateFormat,
		"seq_length":        rule.SeqLength,
		"seq_step":          rule.SeqStep,
		"start_value":       rule.StartValue,
		"max_value":         rule.MaxValue,
		"overflow_strategy": string(rule.OverflowStrategy),
		"reset_cycle":       string(rule.ResetCycle),
		"current_value":     rule.CurrentValue,
		"is_active":         strconv.FormatBool(rule.IsActive),
	}

	if rule.LastResetAt != nil {
		fields["last_reset_at"] = rule.LastResetAt.Format(time.DateTime)
	}

	return s.client.HSet(ctx, rKey, fields).Err()
}
