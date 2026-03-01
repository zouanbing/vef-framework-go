package security

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisNoncePrefix = "vef:security:nonce:"

// RedisNonceStore implements NonceStore using Redis for distributed deployments.
type RedisNonceStore struct {
	client *redis.Client
}

// NewRedisNonceStore creates a new Redis-backed nonce store.
func NewRedisNonceStore(client *redis.Client) NonceStore {
	return &RedisNonceStore{client: client}
}

func (*RedisNonceStore) buildKey(appID, nonce string) string {
	return redisNoncePrefix + appID + ":" + nonce
}

func (s *RedisNonceStore) Exists(ctx context.Context, appID, nonce string) (bool, error) {
	key := s.buildKey(appID, nonce)

	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

func (s *RedisNonceStore) Store(ctx context.Context, appID, nonce string, ttl time.Duration) error {
	key := s.buildKey(appID, nonce)

	return s.client.Set(ctx, key, "1", ttl).Err()
}
