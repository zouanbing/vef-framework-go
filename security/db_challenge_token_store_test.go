package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/result"
)

// TestDBChallengeTokenStoreAutoCreateTable verifies NewDBChallengeTokenStore creates the table automatically.
func TestDBChallengeTokenStoreAutoCreateTable(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)

		require.NoError(t, err, "Should create store without error")
		assert.NotNil(t, store, "Store should not be nil")

		// Calling again should succeed (IF NOT EXISTS).
		store2, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store again without error")
		assert.NotNil(t, store2, "Second store should not be nil")
	})
}

// TestDBChallengeTokenStoreGenerate tests DBChallengeTokenStore Generate scenarios.
func TestDBChallengeTokenStoreGenerate(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("WithPendingAndResolved", func(t *testing.T) {
			principal := NewUser("user1", "Alice", "admin")
			token, err := store.Generate(principal, []string{"totp", "department"}, []string{"sms"})

			require.NoError(t, err, "Should generate token without error")
			assert.NotEmpty(t, token, "Should return a non-empty token")
		})

		t.Run("WithNilResolved", func(t *testing.T) {
			principal := NewUser("user2", "Bob")
			token, err := store.Generate(principal, []string{"totp"}, nil)

			require.NoError(t, err, "Should generate token without error")
			assert.NotEmpty(t, token, "Should return a non-empty token")
		})

		t.Run("WithEmptySlices", func(t *testing.T) {
			principal := NewUser("user3", "Charlie")
			token, err := store.Generate(principal, []string{}, []string{})

			require.NoError(t, err, "Should generate token with empty slices")
			assert.NotEmpty(t, token, "Should return a non-empty token")
		})

		t.Run("WithDetails", func(t *testing.T) {
			principal := NewUser("user4", "Diana", "editor")
			principal.Details = map[string]any{"department": "engineering", "level": 3}

			token, err := store.Generate(principal, []string{"totp"}, nil)

			require.NoError(t, err, "Should generate token with details")
			assert.NotEmpty(t, token, "Should return a non-empty token")
		})

		t.Run("WithNoRoles", func(t *testing.T) {
			principal := NewUser("user5", "Eve")
			token, err := store.Generate(principal, []string{"totp"}, nil)

			require.NoError(t, err, "Should generate token without roles")
			assert.NotEmpty(t, token, "Should return a non-empty token")
		})
	})
}

// TestDBChallengeTokenStoreParse tests DBChallengeTokenStore Parse scenarios.
func TestDBChallengeTokenStoreParse(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("RoundTripWithFullState", func(t *testing.T) {
			principal := NewUser("user1", "Alice", "admin", "editor")
			pending := []string{"totp", "department"}
			resolved := []string{"sms"}

			token, err := store.Generate(principal, pending, resolved)
			require.NoError(t, err, "Should generate token without error")

			state, err := store.Parse(token)
			require.NoError(t, err, "Should parse token without error")
			require.NotNil(t, state, "Should return non-nil state")

			assert.Equal(t, "user1", state.Principal.ID, "Should preserve principal ID")
			assert.Equal(t, "Alice", state.Principal.Name, "Should preserve principal name")
			assert.Equal(t, PrincipalTypeUser, state.Principal.Type, "Should create user principal")
			assert.Equal(t, []string{"admin", "editor"}, state.Principal.Roles, "Should preserve roles")
			assert.Equal(t, pending, state.Pending, "Should preserve pending list")
			assert.Equal(t, resolved, state.Resolved, "Should preserve resolved list")
		})

		t.Run("WithNilResolved", func(t *testing.T) {
			principal := NewUser("user2", "Bob")
			token, err := store.Generate(principal, []string{"totp"}, nil)
			require.NoError(t, err, "Should generate token without error")

			state, err := store.Parse(token)
			require.NoError(t, err, "Should parse token without error")
			require.NotNil(t, state, "Should return non-nil state")

			assert.Equal(t, []string{"totp"}, state.Pending, "Should preserve pending list")
			assert.Empty(t, state.Resolved, "Should have empty resolved list")
		})

		t.Run("WithNoRoles", func(t *testing.T) {
			principal := NewUser("user3", "Charlie")
			token, err := store.Generate(principal, []string{"totp"}, nil)
			require.NoError(t, err, "Should generate token without error")

			state, err := store.Parse(token)
			require.NoError(t, err, "Should parse token without error")
			require.NotNil(t, state, "Should return non-nil state")

			assert.Empty(t, state.Principal.Roles, "Should have empty roles")
		})

		t.Run("WithDetails", func(t *testing.T) {
			principal := NewUser("user4", "Diana", "admin")
			principal.Details = map[string]any{"department": "engineering"}

			token, err := store.Generate(principal, []string{"totp"}, nil)
			require.NoError(t, err, "Should generate token without error")

			state, err := store.Parse(token)
			require.NoError(t, err, "Should parse token without error")
			require.NotNil(t, state, "Should return non-nil state")

			assert.NotNil(t, state.Principal.Details, "Should preserve details")
		})

		t.Run("SubjectWithAtSignInName", func(t *testing.T) {
			principal := NewUser("user5", "user@example.com")
			token, err := store.Generate(principal, []string{"totp"}, nil)
			require.NoError(t, err, "Should generate token without error")

			state, err := store.Parse(token)
			require.NoError(t, err, "Should parse token without error")
			require.NotNil(t, state, "Should return non-nil state")

			assert.Equal(t, "user5", state.Principal.ID, "Should preserve principal ID")
			assert.Equal(t, "user@example.com", state.Principal.Name, "Should preserve name with @ sign")
		})

		t.Run("RejectsEmptyToken", func(t *testing.T) {
			_, err := store.Parse("")

			assert.Error(t, err, "Should reject empty token")
			assert.ErrorIs(t, err, result.ErrTokenInvalid, "Should return token invalid error")
		})

		t.Run("RejectsNonExistentToken", func(t *testing.T) {
			_, err := store.Parse("non-existent-token-value")

			assert.Error(t, err, "Should reject non-existent token")
			assert.ErrorIs(t, err, result.ErrTokenInvalid, "Should return token invalid error")
		})
	})
}

// TestDBChallengeTokenStoreExpiration tests DBChallengeTokenStore expiration scenarios.
func TestDBChallengeTokenStoreExpiration(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("ExpiredTokenNotFound", func(t *testing.T) {
			// Insert a record that is already expired.
			_, err := env.DB.NewInsert().Model(&ChallengeRecord{
				Token:     "expired-challenge-token",
				UserID:    "user-expired",
				UserName:  "ExpiredUser",
				Roles:     []string{"admin"},
				Pending:   []string{"totp"},
				ExpiresAt: time.Now().Add(-1 * time.Minute),
			}).Exec(env.Ctx)
			require.NoError(t, err, "Should insert expired record without error")

			_, err = store.Parse("expired-challenge-token")

			assert.Error(t, err, "Should reject expired token")
			assert.ErrorIs(t, err, result.ErrTokenInvalid, "Should return token invalid error for expired token")
		})

		t.Run("NonExpiredTokenFound", func(t *testing.T) {
			// Insert a record with future expiration.
			_, err := env.DB.NewInsert().Model(&ChallengeRecord{
				Token:     "valid-challenge-token",
				UserID:    "user-valid",
				UserName:  "ValidUser",
				Roles:     []string{"editor"},
				Pending:   []string{"department"},
				Resolved:  []string{"sms"},
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}).Exec(env.Ctx)
			require.NoError(t, err, "Should insert valid record without error")

			state, err := store.Parse("valid-challenge-token")

			require.NoError(t, err, "Should parse non-expired token without error")
			require.NotNil(t, state, "Should return non-nil state")
			assert.Equal(t, "user-valid", state.Principal.ID, "Should preserve principal ID")
			assert.Equal(t, "ValidUser", state.Principal.Name, "Should preserve principal name")
			assert.Equal(t, []string{"editor"}, state.Principal.Roles, "Should preserve roles")
			assert.Equal(t, []string{"department"}, state.Pending, "Should preserve pending list")
			assert.Equal(t, []string{"sms"}, state.Resolved, "Should preserve resolved list")
		})
	})
}
