package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
)

type mockQueryAction struct{ cqrs.QueryBase }
type mockCommandAction struct{ cqrs.CommandBase }

func TestTransactionBehavior(t *testing.T) {
	t.Run("SkipsQueryActions", func(t *testing.T) {
		b := &TransactionBehavior{}
		query := &mockQueryAction{}

		called := false
		_, err := b.Handle(context.Background(), query, func(ctx context.Context) (any, error) {
			called = true
			db := contextx.DB(ctx)
			assert.Nil(t, db, "Query should not have DB in context")
			return "result", nil
		})

		require.NoError(t, err)
		assert.True(t, called)
	})
}
