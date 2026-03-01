package security

import (
	"fmt"
	"sync"
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
			details, ok := state.Principal.Details.(map[string]any)
			require.True(t, ok, "Details should be map[string]any")
			assert.Equal(t, "engineering", details["department"], "Should preserve detail values")
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

			require.Error(t, err, "Should reject empty token")

			resErr, ok := result.AsErr(err)
			require.True(t, ok, "Should return a result.Error")
			assert.Equal(t, result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
		})

		t.Run("RejectsNonExistentToken", func(t *testing.T) {
			_, err := store.Parse("non-existent-token-value")

			require.Error(t, err, "Should reject non-existent token")

			resErr, ok := result.AsErr(err)
			require.True(t, ok, "Should return a result.Error")
			assert.Equal(t, result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
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

			require.Error(t, err, "Should reject expired token")

			resErr, ok := result.AsErr(err)
			require.True(t, ok, "Should return a result.Error")
			assert.Equal(t, result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
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

// TestDBChallengeTokenStoreTokenUniqueness tests that multiple generates produce unique tokens.
func TestDBChallengeTokenStoreTokenUniqueness(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("MultipleGeneratesProduceUniqueTokens", func(t *testing.T) {
			principal := NewUser("user1", "Alice", "admin")

			tokens := make(map[string]struct{}, 50)
			for range 50 {
				token, genErr := store.Generate(principal, []string{"totp"}, nil)
				require.NoError(t, genErr, "Should generate token without error")

				tokens[token] = struct{}{}
			}

			assert.Len(t, tokens, 50, "All 50 tokens should be unique")
		})
	})
}

// TestDBChallengeTokenStoreConcurrency tests DBChallengeTokenStore concurrent access safety.
func TestDBChallengeTokenStoreConcurrency(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBChallengeTokenStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("ConcurrentGenerateAndParse", func(t *testing.T) {
			var wg sync.WaitGroup

			numGoroutines := 50

			for i := range numGoroutines {
				wg.Go(func() {
					principal := NewUser(
						fmt.Sprintf("user-%d", i),
						fmt.Sprintf("Name-%d", i),
						"admin",
					)

					token, genErr := store.Generate(principal, []string{"totp"}, []string{"sms"})
					assert.NoError(t, genErr, "Should generate token without error in goroutine %d", i)
					assert.NotEmpty(t, token, "Should return non-empty token in goroutine %d", i)

					state, parseErr := store.Parse(token)
					assert.NoError(t, parseErr, "Should parse token without error in goroutine %d", i)
					assert.NotNil(t, state, "Should return non-nil state in goroutine %d", i)
				})
			}

			wg.Wait()
		})
	})
}
