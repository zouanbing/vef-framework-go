package orm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextWithDB(t *testing.T) {
	t.Run("StoreAndRetrieve", func(t *testing.T) {
		db := &BunDB{} // non-nil DB for context storage
		ctx := ContextWithDB(context.Background(), db)
		got, ok := DBFromContext(ctx)

		assert.True(t, ok, "Should find DB in context")
		assert.Same(t, db, got, "Should return the same DB")
	})

	t.Run("NotPresent", func(t *testing.T) {
		got, ok := DBFromContext(context.Background())

		assert.False(t, ok, "Should not find DB in empty context")
		assert.Nil(t, got, "Should return nil when not present")
	})
}
