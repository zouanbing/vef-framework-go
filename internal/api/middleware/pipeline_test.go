package middleware

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"github.com/ilxqx/vef-framework-go/api"
)

// mockMiddleware is a test implementation of api.Middleware.
type mockMiddleware struct {
	name  string
	order int
}

func (m *mockMiddleware) Name() string              { return m.name }
func (m *mockMiddleware) Order() int                { return m.order }
func (*mockMiddleware) Process(ctx fiber.Ctx) error { return ctx.Next() }

var _ api.Middleware = (*mockMiddleware)(nil)

func TestNewChain(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		chain := NewChain()
		assert.NotNil(t, chain)
		assert.Empty(t, chain.Handlers())
	})

	t.Run("Single", func(t *testing.T) {
		chain := NewChain(&mockMiddleware{name: "test", order: 1})
		handlers := chain.Handlers()

		assert.Len(t, handlers, 1)
		assert.NotNil(t, handlers[0])
	})

	t.Run("SortsByOrder", func(t *testing.T) {
		high := &mockMiddleware{name: "high", order: 100}
		low := &mockMiddleware{name: "low", order: -100}
		mid := &mockMiddleware{name: "mid", order: 0}

		chain := NewChain(high, low, mid)

		// Verify internal ordering: low(-100) < mid(0) < high(100)
		assert.Equal(t, "low", chain.middlewares[0].Name())
		assert.Equal(t, "mid", chain.middlewares[1].Name())
		assert.Equal(t, "high", chain.middlewares[2].Name())
	})

	t.Run("DoesNotMutateInput", func(t *testing.T) {
		mids := []api.Middleware{
			&mockMiddleware{name: "b", order: 2},
			&mockMiddleware{name: "a", order: 1},
		}

		NewChain(mids...)

		// Original slice should be unmodified
		assert.Equal(t, "b", mids[0].Name())
		assert.Equal(t, "a", mids[1].Name())
	})
}

func TestChainHandlers(t *testing.T) {
	t.Run("ReturnsCorrectCount", func(t *testing.T) {
		chain := NewChain(
			&mockMiddleware{name: "a", order: 1},
			&mockMiddleware{name: "b", order: 2},
		)

		assert.Len(t, chain.Handlers(), 2)
	})

	t.Run("HandlerTypesAreFiberHandler", func(t *testing.T) {
		chain := NewChain(&mockMiddleware{name: "test", order: 1})

		for i, h := range chain.Handlers() {
			_, ok := h.(func(fiber.Ctx) error)
			assert.True(t, ok, "Handler %d should be func(fiber.Ctx) error", i)
		}
	})

	t.Run("TypicalMiddlewareOrdering", func(t *testing.T) {
		auth := &mockMiddleware{name: "auth", order: -100}
		contextual := &mockMiddleware{name: "contextual", order: -90}
		dataPermission := &mockMiddleware{name: "data_permission", order: -80}
		rateLimit := &mockMiddleware{name: "ratelimit", order: -70}
		audit := &mockMiddleware{name: "audit", order: -60}

		// Pass in shuffled order
		chain := NewChain(audit, auth, rateLimit, contextual, dataPermission)

		assert.Equal(t, "auth", chain.middlewares[0].Name())
		assert.Equal(t, "contextual", chain.middlewares[1].Name())
		assert.Equal(t, "data_permission", chain.middlewares[2].Name())
		assert.Equal(t, "ratelimit", chain.middlewares[3].Name())
		assert.Equal(t, "audit", chain.middlewares[4].Name())
	})
}
