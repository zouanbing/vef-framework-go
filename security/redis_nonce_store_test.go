package security

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/internal/testx"
)

type RedisNonceStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     NonceStore
}

func (s *RedisNonceStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.store = NewRedisNonceStore(s.client)
}

func (s *RedisNonceStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisNonceStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

// --- Exists ---

func (s *RedisNonceStoreTestSuite) TestExists() {
	ctx := context.Background()

	s.Run("NonExistentNonce", func() {
		exists, err := s.store.Exists(ctx, "test-app", "test-nonce")

		s.Require().NoError(err, "Should check existence without error")
		s.False(exists, "Nonce should not exist initially")
	})

	s.Run("ExistingNonce", func() {
		err := s.store.Store(ctx, "test-app", "test-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store nonce without error")

		exists, err := s.store.Exists(ctx, "test-app", "test-nonce")

		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Nonce should exist after storing")
	})

	s.Run("EmptyAppID", func() {
		err := s.store.Store(ctx, "", "test-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store nonce with empty appID without error")

		exists, err := s.store.Exists(ctx, "", "test-nonce")

		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Should handle empty appID")
	})

	s.Run("EmptyNonce", func() {
		err := s.store.Store(ctx, "test-app", "", 5*time.Minute)
		s.Require().NoError(err, "Should store empty nonce without error")

		exists, err := s.store.Exists(ctx, "test-app", "")

		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Should handle empty nonce")
	})
}

// --- Store ---

func (s *RedisNonceStoreTestSuite) TestStore() {
	ctx := context.Background()

	s.Run("StoreNewNonce", func() {
		err := s.store.Store(ctx, "test-app", "test-nonce", 5*time.Minute)

		s.Require().NoError(err, "Should store new nonce without error")

		exists, err := s.store.Exists(ctx, "test-app", "test-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Stored nonce should exist")
	})

	s.Run("StoreDuplicateNonce", func() {
		err := s.store.Store(ctx, "test-app", "test-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store first nonce without error")

		err = s.store.Store(ctx, "test-app", "test-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store duplicate nonce without error")

		exists, err := s.store.Exists(ctx, "test-app", "test-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Duplicate nonce should still exist")
	})
}

// --- DifferentApps ---

func (s *RedisNonceStoreTestSuite) TestDifferentApps() {
	ctx := context.Background()

	s.Run("SameNonceDifferentApps", func() {
		err := s.store.Store(ctx, "test-app-1", "shared-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store nonce for first app without error")

		exists, err := s.store.Exists(ctx, "test-app-2", "shared-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.False(exists, "Same nonce for different app should not exist")

		exists, err = s.store.Exists(ctx, "test-app-1", "shared-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Original app should still have the nonce")
	})

	s.Run("DifferentNoncesSameApp", func() {
		err := s.store.Store(ctx, "test-app-3", "nonce-1", 5*time.Minute)
		s.Require().NoError(err, "Should store first nonce without error")

		exists, err := s.store.Exists(ctx, "test-app-3", "nonce-2")
		s.Require().NoError(err, "Should check existence without error")
		s.False(exists, "Different nonce for same app should not exist")

		exists, err = s.store.Exists(ctx, "test-app-3", "nonce-1")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Original nonce should still exist")
	})

	s.Run("MultipleAppsMultipleNonces", func() {
		apps := []string{"app-a", "app-b", "app-c"}
		nonces := []string{"nonce-x", "nonce-y", "nonce-z"}

		for _, app := range apps {
			for _, nonce := range nonces {
				err := s.store.Store(ctx, app, nonce, 5*time.Minute)
				s.Require().NoError(err, "Should store nonce %s for app %s without error", nonce, app)
			}
		}

		for _, app := range apps {
			for _, nonce := range nonces {
				exists, err := s.store.Exists(ctx, app, nonce)
				s.Require().NoError(err, "Should check existence without error")
				s.True(exists, "Nonce %s for app %s should exist", nonce, app)
			}
		}
	})
}

// --- TTL Expiration ---

func (s *RedisNonceStoreTestSuite) TestTTLExpiration() {
	ctx := context.Background()

	s.Run("NonceExpiresAfterTTL", func() {
		err := s.store.Store(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		s.Require().NoError(err, "Should store nonce without error")

		exists, err := s.store.Exists(ctx, "test-app", "expiring-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Nonce should exist immediately after storing")

		time.Sleep(100 * time.Millisecond)

		exists, err = s.store.Exists(ctx, "test-app", "expiring-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.False(exists, "Nonce should not exist after TTL expiration")
	})

	s.Run("DifferentTTLs", func() {
		err := s.store.Store(ctx, "test-app", "short-ttl", 50*time.Millisecond)
		s.Require().NoError(err, "Should store short TTL nonce without error")

		err = s.store.Store(ctx, "test-app", "long-ttl", 5*time.Minute)
		s.Require().NoError(err, "Should store long TTL nonce without error")

		time.Sleep(100 * time.Millisecond)

		exists, err := s.store.Exists(ctx, "test-app", "short-ttl")
		s.Require().NoError(err, "Should check existence without error")
		s.False(exists, "Short TTL nonce should have expired")

		exists, err = s.store.Exists(ctx, "test-app", "long-ttl")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Long TTL nonce should still exist")
	})

	s.Run("NonceExistsBeforeTTL", func() {
		err := s.store.Store(ctx, "test-app", "test-nonce", 1*time.Second)
		s.Require().NoError(err, "Should store nonce without error")

		time.Sleep(100 * time.Millisecond)

		exists, err := s.store.Exists(ctx, "test-app", "test-nonce")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Nonce should still exist before TTL expires")
	})
}

// --- Key Format ---

func (s *RedisNonceStoreTestSuite) TestKeyFormat() {
	ctx := context.Background()

	s.Run("SpecialCharactersInAppID", func() {
		specialAppIDs := []string{
			"app:with:colons",
			"app-with-dashes",
			"app_with_underscores",
			"app.with.dots",
			"app/with/slashes",
		}

		for _, appID := range specialAppIDs {
			err := s.store.Store(ctx, appID, "test-nonce", 5*time.Minute)
			s.Require().NoError(err, "Should store nonce for appID %s without error", appID)

			exists, err := s.store.Exists(ctx, appID, "test-nonce")
			s.Require().NoError(err, "Should check existence without error")
			s.True(exists, "Should handle appID: %s", appID)
		}
	})

	s.Run("SpecialCharactersInNonce", func() {
		specialNonces := []string{
			"nonce:with:colons",
			"nonce-with-dashes",
			"nonce_with_underscores",
			"nonce.with.dots",
			"nonce/with/slashes",
		}

		for _, nonce := range specialNonces {
			err := s.store.Store(ctx, "test-app", nonce, 5*time.Minute)
			s.Require().NoError(err, "Should store nonce %s without error", nonce)

			exists, err := s.store.Exists(ctx, "test-app", nonce)
			s.Require().NoError(err, "Should check existence without error")
			s.True(exists, "Should handle nonce: %s", nonce)
		}
	})

	s.Run("UnicodeCharacters", func() {
		err := s.store.Store(ctx, "应用", "随机数", 5*time.Minute)
		s.Require().NoError(err, "Should store nonce with unicode characters without error")

		exists, err := s.store.Exists(ctx, "应用", "随机数")
		s.Require().NoError(err, "Should check existence without error")
		s.True(exists, "Should handle unicode characters")
	})
}

// --- Concurrency ---

func (s *RedisNonceStoreTestSuite) TestConcurrency() {
	ctx := context.Background()

	s.Run("ConcurrentStoreAndExists", func() {
		var wg sync.WaitGroup

		numGoroutines := 100

		for id := range numGoroutines {
			wg.Go(func() {
				appID := "test-app"
				nonce := fmt.Sprintf("nonce-%d", id)

				err := s.store.Store(ctx, appID, nonce, 5*time.Minute)
				s.Require().NoError(err, "Should store nonce without error in goroutine %d", id)

				_, err = s.store.Exists(ctx, appID, nonce)
				s.Require().NoError(err, "Should check existence without error in goroutine %d", id)
			})
		}

		wg.Wait()
	})

	s.Run("ConcurrentDifferentApps", func() {
		var wg sync.WaitGroup

		numApps := 50

		for id := range numApps {
			wg.Go(func() {
				appID := fmt.Sprintf("app-%d", id)
				nonce := fmt.Sprintf("nonce-%d", id)

				err := s.store.Store(ctx, appID, nonce, 5*time.Minute)
				s.Require().NoError(err, "Should store nonce without error for app %s", appID)

				exists, err := s.store.Exists(ctx, appID, nonce)
				s.Require().NoError(err, "Should check existence without error for app %s", appID)
				s.True(exists, "Nonce should exist for app %s", appID)
			})
		}

		wg.Wait()
	})
}

// TestRedisNonceStoreTestSuite runs the Redis nonce store test suite.
func TestRedisNonceStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisNonceStoreTestSuite))
}
