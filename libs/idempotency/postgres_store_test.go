package idempotency

import (
	"context"
	"database/sql"
	"idempotency/common"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestPostgresDB(t *testing.T) *sql.DB {
	// Replace these with your PostgreSQL connection details
	dsn := "postgres://postgres:@localhost:5432/testdb?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	// Clean up existing table if it exists
	_, err = db.Exec("DROP TABLE IF EXISTS idempotency_keys")
	require.NoError(t, err)

	return db
}

func TestPostgresStore(t *testing.T) {
	db := setupTestPostgresDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	t.Run("Set and Get key", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "test-key"
		expiration := time.Now().Add(time.Hour)
		now := time.Now()

		// Set the key
		status, err := store.Set(ctx, namespace, key, &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status)

		// Get the key
		result, err := store.Get(ctx, namespace, key)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, namespace, result.Namespace)
		assert.Equal(t, key, result.Key)
		assert.NotNil(t, result.ExpiresAt)
		assert.True(t, result.ExpiresAt.After(now))
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		ctx := context.Background()
		result, err := store.Get(ctx, "non-existent", "key")
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Set with expired key", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "expired-key"
		expiration := time.Now().Add(-time.Hour) // Past time for expired key

		// Set the key with past expiration
		status, err := store.Set(ctx, namespace, key, &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status)

		// Try to get the expired key
		result, err := store.Get(ctx, namespace, key)
		require.NoError(t, err)
		assert.Nil(t, result) // Should return nil for expired keys
	})

	t.Run("Set duplicate key", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "duplicate-key"
		expiration := time.Now().Add(time.Hour)

		// Set initial key
		status1, err := store.Set(ctx, namespace, key, &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status1)

		// Try to set the same key again
		status2, err := store.Set(ctx, namespace, key, &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.DUPLICATE, status2)
	})

	t.Run("Set non-expiring key", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "non-expiring-key"

		// Set key without expiration
		status, err := store.Set(ctx, namespace, key, nil)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status)

		// Get the key
		result, err := store.Get(ctx, namespace, key)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.ExpiresAt)
	})
}

func TestPostgresStoreFactory(t *testing.T) {
	db := setupTestPostgresDB(t)
	defer db.Close()

	t.Run("Create store successfully", func(t *testing.T) {
		factory := NewPostgresStoreFactory(db)
		store, err := factory.CreateStore()
		require.NoError(t, err)
		assert.NotNil(t, store)
	})

	t.Run("Create store with nil db", func(t *testing.T) {
		factory := NewPostgresStoreFactory(nil)
		store, err := factory.CreateStore()
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "postgres connection is required")
	})
}

func TestPostgresStoreConcurrent(t *testing.T) {
	db := setupTestPostgresDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	t.Run("Concurrent Set operations", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "concurrent-key"
		expiration := time.Now().Add(time.Hour)

		// Perform concurrent Set operations
		concurrency := 10
		results := make(chan string, concurrency)
		errors := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				status, err := store.Set(ctx, namespace, key, &expiration)
				results <- status
				errors <- err
			}()
		}

		// Check results
		successCount := 0
		for i := 0; i < concurrency; i++ {
			err := <-errors
			require.NoError(t, err)

			status := <-results
			if status == common.SUCCEEDED {
				successCount++
			}
		}

		// Only one operation should succeed
		assert.Equal(t, 1, successCount)

		// Verify final state
		result, err := store.Get(ctx, namespace, key)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

func TestPostgresStoreTransactionRollback(t *testing.T) {
	db := setupTestPostgresDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	t.Run("Transaction rollback on error", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "rollback-key"
		expiration := time.Now().Add(time.Hour)

		// Force an error by closing the database connection
		db.Close()

		// Attempt to set a key
		status, err := store.Set(ctx, namespace, key, &expiration)
		assert.Error(t, err)
		assert.Equal(t, common.FAILED, status)

		// Reconnect to check state
		db = setupTestPostgresDB(t)
		store, err = NewPostgresStore(db)
		require.NoError(t, err)

		// Verify key was not set
		result, err := store.Get(ctx, namespace, key)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}
