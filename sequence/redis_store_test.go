package sequence

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type RedisStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     *RedisStore
}

func (s *RedisStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.store = NewRedisStore(s.client).(*RedisStore)
}

func (s *RedisStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

// --- Load ---

func (s *RedisStoreTestSuite) TestLoad() {
	ctx := context.Background()

	s.Run("ExistingActiveRule", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, SeqStep: 1, SeqLength: 4}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		loaded, err := s.store.Load(ctx, "order")

		s.Require().NoError(err)
		s.Equal("order", loaded.Key)
		s.Equal("Order No", loaded.Name)
		s.Equal(1, loaded.SeqStep)
		s.Equal(4, loaded.SeqLength)
		s.True(loaded.IsActive)
	})

	s.Run("NonExistentKey", func() {
		_, err := s.store.Load(ctx, "non-existent")

		s.ErrorIs(err, ErrRuleNotFound)
	})

	s.Run("InactiveRule", func() {
		rule := &Rule{Key: "inactive", Name: "Inactive Rule", IsActive: false}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		_, err := s.store.Load(ctx, "inactive")

		s.ErrorIs(err, ErrRuleNotFound)
	})

	s.Run("RuleWithAllFields", func() {
		rule := &Rule{
			Key:              "full",
			Name:             "Full Rule",
			Prefix:           "PRE-",
			Suffix:           "-SUF",
			DateFormat:       "yyyyMMdd",
			SeqLength:        6,
			SeqStep:          2,
			StartValue:       100,
			MaxValue:         9999,
			OverflowStrategy: OverflowReset,
			ResetCycle:       ResetDaily,
			CurrentValue:     500,
			IsActive:         true,
		}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		loaded, err := s.store.Load(ctx, "full")

		s.Require().NoError(err)
		s.Equal("PRE-", loaded.Prefix)
		s.Equal("-SUF", loaded.Suffix)
		s.Equal("yyyyMMdd", loaded.DateFormat)
		s.Equal(6, loaded.SeqLength)
		s.Equal(2, loaded.SeqStep)
		s.Equal(100, loaded.StartValue)
		s.Equal(9999, loaded.MaxValue)
		s.Equal(OverflowReset, loaded.OverflowStrategy)
		s.Equal(ResetDaily, loaded.ResetCycle)
		s.Equal(500, loaded.CurrentValue)
	})
}

// --- Increment ---

func (s *RedisStoreTestSuite) TestIncrement() {
	ctx := context.Background()

	s.Run("BasicIncrement", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, SeqStep: 1, SeqLength: 4}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		newVal, err := s.store.Increment(ctx, "order", 1, 1, 0, false)

		s.Require().NoError(err)
		s.Equal(1, newVal)
	})

	s.Run("IncrementByStep", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		newVal, err := s.store.Increment(ctx, "order", 5, 1, 0, false)

		s.Require().NoError(err)
		s.Equal(5, newVal)
	})

	s.Run("IncrementBatch", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		newVal, err := s.store.Increment(ctx, "order", 1, 3, 0, false)

		s.Require().NoError(err)
		s.Equal(3, newVal)
	})

	s.Run("ConsecutiveIncrements", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		val1, err := s.store.Increment(ctx, "order", 1, 1, 0, false)
		s.Require().NoError(err)
		s.Equal(1, val1)

		val2, err := s.store.Increment(ctx, "order", 1, 1, 0, false)
		s.Require().NoError(err)
		s.Equal(2, val2)
	})

	s.Run("NonExistentKey", func() {
		_, err := s.store.Increment(ctx, "non-existent", 1, 1, 0, false)

		s.ErrorIs(err, ErrRuleNotFound)
	})

	s.Run("ResetAndIncrement", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		newVal, err := s.store.Increment(ctx, "order", 1, 1, 0, true)

		s.Require().NoError(err)
		s.Equal(1, newVal)
	})

	s.Run("ResetWithStartValue", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 100}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		newVal, err := s.store.Increment(ctx, "order", 1, 1, 1000, true)

		s.Require().NoError(err)
		s.Equal(1001, newVal)
	})
}

// --- Concurrency ---

func (s *RedisStoreTestSuite) TestConcurrency() {
	ctx := context.Background()

	s.Run("ConcurrentIncrements", func() {
		rule := &Rule{Key: "order", Name: "Order No", IsActive: true, CurrentValue: 0}
		s.Require().NoError(s.store.RegisterRule(ctx, rule))

		numGoroutines := 100
		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {
				_, err := s.store.Increment(ctx, "order", 1, 1, 0, false)
				s.Require().NoError(err)
			})
		}
		wg.Wait()

		// Verify final value
		loaded, err := s.store.Load(ctx, "order")
		s.Require().NoError(err)
		s.Equal(numGoroutines, loaded.CurrentValue)
	})
}

func TestRedisStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisStoreTestSuite))
}
