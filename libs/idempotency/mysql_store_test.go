package idempotency

import (
	"context"
	"database/sql"
	"idempotency/common"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Replace these with your MySQL connection details
	dsn := "root:@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	// Clean up existing table if it exists
	_, err = db.Exec("DROP TABLE IF EXISTS idempotency_keys")
	require.NoError(t, err)

	return db
}

func TestMySQLStore(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store, err := NewMySQLStore(db)
	require.NoError(t, err)

	t.Run("Set and Get key", func(t *testing.T) {
		ctx := context.Background()
		namespace := "test-namespace"
		key := "test-key"
		expiration := time.Now().Add(time.Hour)

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
		assert.True(t, result.ExpiresAt.After(time.Now()))
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

func TestMySQLStoreFactory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("Create store successfully", func(t *testing.T) {
		factory := NewMySQLStoreFactory(db)
		store, err := factory.CreateStore()
		require.NoError(t, err)
		assert.NotNil(t, store)
	})

	t.Run("Create store with nil db", func(t *testing.T) {
		factory := NewMySQLStoreFactory(nil)
		store, err := factory.CreateStore()
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "mysql connection is required")
	})
}
