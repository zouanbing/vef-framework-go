package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/internal/testx"
)

// TestDBNonceStoreAutoCreateTable verifies NewDBNonceStore creates the table automatically.
func TestDBNonceStoreAutoCreateTable(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBNonceStore(env.Ctx, env.DB)

		require.NoError(t, err, "Should create store without error")
		assert.NotNil(t, store, "Store should not be nil")

		// Calling again should succeed (IF NOT EXISTS).
		store2, err := NewDBNonceStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store again without error")
		assert.NotNil(t, store2, "Second store should not be nil")
	})
}

// TestDBNonceStoreExists tests DBNonceStore Exists scenarios.
func TestDBNonceStoreExists(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBNonceStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("NonExistentNonce", func(t *testing.T) {
			exists, err := store.Exists(env.Ctx, "test-app", "test-nonce")

			require.NoError(t, err, "Should check existence without error")
			assert.False(t, exists, "Nonce should not exist initially")
		})

		t.Run("ExistingNonce", func(t *testing.T) {
			err := store.Store(env.Ctx, "exist-app", "exist-nonce", 5*time.Minute)
			require.NoError(t, err, "Should store nonce without error")

			exists, err := store.Exists(env.Ctx, "exist-app", "exist-nonce")

			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Nonce should exist after storing")
		})

		t.Run("EmptyAppID", func(t *testing.T) {
			err := store.Store(env.Ctx, "", "empty-app-nonce", 5*time.Minute)
			require.NoError(t, err, "Should store nonce with empty appID without error")

			exists, err := store.Exists(env.Ctx, "", "empty-app-nonce")

			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Should handle empty appID")
		})

		t.Run("EmptyNonce", func(t *testing.T) {
			err := store.Store(env.Ctx, "empty-nonce-app", "", 5*time.Minute)
			require.NoError(t, err, "Should store empty nonce without error")

			exists, err := store.Exists(env.Ctx, "empty-nonce-app", "")

			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Should handle empty nonce")
		})
	})
}

// TestDBNonceStoreStore tests DBNonceStore Store scenarios.
func TestDBNonceStoreStore(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBNonceStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("StoreNewNonce", func(t *testing.T) {
			err := store.Store(env.Ctx, "store-app", "new-nonce", 5*time.Minute)

			require.NoError(t, err, "Should store new nonce without error")

			exists, err := store.Exists(env.Ctx, "store-app", "new-nonce")
			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Stored nonce should exist")
		})

		t.Run("StoreDuplicateNonce", func(t *testing.T) {
			err := store.Store(env.Ctx, "dup-app", "dup-nonce", 5*time.Minute)
			require.NoError(t, err, "Should store first nonce without error")

			err = store.Store(env.Ctx, "dup-app", "dup-nonce", 5*time.Minute)
			require.NoError(t, err, "Should store duplicate nonce without error")

			exists, err := store.Exists(env.Ctx, "dup-app", "dup-nonce")
			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Duplicate nonce should still exist")
		})
	})
}

// TestDBNonceStoreDifferentApps tests DBNonceStore different apps scenarios.
func TestDBNonceStoreDifferentApps(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBNonceStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("SameNonceDifferentApps", func(t *testing.T) {
			err := store.Store(env.Ctx, "diff-app-1", "shared-nonce", 5*time.Minute)
			require.NoError(t, err, "Should store nonce for first app without error")

			exists, err := store.Exists(env.Ctx, "diff-app-2", "shared-nonce")
			require.NoError(t, err, "Should check existence without error")
			assert.False(t, exists, "Same nonce for different app should not exist")

			exists, err = store.Exists(env.Ctx, "diff-app-1", "shared-nonce")
			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Original app should still have the nonce")
		})

		t.Run("DifferentNoncesSameApp", func(t *testing.T) {
			err := store.Store(env.Ctx, "same-app", "nonce-1", 5*time.Minute)
			require.NoError(t, err, "Should store first nonce without error")

			exists, err := store.Exists(env.Ctx, "same-app", "nonce-2")
			require.NoError(t, err, "Should check existence without error")
			assert.False(t, exists, "Different nonce for same app should not exist")

			exists, err = store.Exists(env.Ctx, "same-app", "nonce-1")
			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Original nonce should still exist")
		})
	})
}

// TestDBNonceStoreExpiration tests DBNonceStore expiration scenarios.
func TestDBNonceStoreExpiration(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store, err := NewDBNonceStore(env.Ctx, env.DB)
		require.NoError(t, err, "Should create store without error")

		t.Run("ExpiredNonceNotFound", func(t *testing.T) {
			// Insert a record that is already expired.
			_, err := env.DB.NewInsert().Model(&NonceRecord{
				AppID:     "expire-app",
				Nonce:     "expired-nonce",
				ExpiresAt: time.Now().Add(-1 * time.Minute),
			}).Exec(env.Ctx)
			require.NoError(t, err, "Should insert expired record without error")

			exists, err := store.Exists(env.Ctx, "expire-app", "expired-nonce")

			require.NoError(t, err, "Should check existence without error")
			assert.False(t, exists, "Expired nonce should not be found")
		})

		t.Run("NonExpiredNonceFound", func(t *testing.T) {
			// Insert a record with future expiration.
			_, err := env.DB.NewInsert().Model(&NonceRecord{
				AppID:     "valid-app",
				Nonce:     "valid-nonce",
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}).Exec(env.Ctx)
			require.NoError(t, err, "Should insert valid record without error")

			exists, err := store.Exists(env.Ctx, "valid-app", "valid-nonce")

			require.NoError(t, err, "Should check existence without error")
			assert.True(t, exists, "Non-expired nonce should be found")
		})
	})
}
