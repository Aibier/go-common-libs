package idempotency

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name           string
	namespace      string
	key            string
	expiration     *time.Time
	expectedStatus string
	expectError    bool
}

func setupTestDatabases(t *testing.T) (map[StoreType]*sql.DB, func()) {
	dbs := make(map[StoreType]*sql.DB)

	// Setup MySQL
	mysqlDB, err := sql.Open("mysql", "root:@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true")
	require.NoError(t, err)
	require.NoError(t, mysqlDB.Ping())
	_, err = mysqlDB.Exec("DROP TABLE IF EXISTS idempotency_keys")
	require.NoError(t, err)
	dbs[StoreTypeMySQL] = mysqlDB

	// Setup PostgreSQL
	postgresDB, err := sql.Open("postgres", "postgres://postgres:@localhost:5432/testdb?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, postgresDB.Ping())
	_, err = postgresDB.Exec("DROP TABLE IF EXISTS idempotency_keys")
	require.NoError(t, err)
	dbs[StoreTypePostgres] = postgresDB

	cleanup := func() {
		for _, db := range dbs {
			db.Close()
		}
	}

	return dbs, cleanup
}

func TestSetKeyE2E(t *testing.T) {
	dbs, cleanup := setupTestDatabases(t)
	defer cleanup()

	now := time.Now().UTC()

	testCases := []testCase{
		{
			name:           "successful set key",
			namespace:      "test-namespace",
			key:            "test-key-1",
			expiration:     ptr(time.Now().UTC().Add(1 * time.Hour)),
			expectedStatus: "SUCCEEDED",
			expectError:    false,
		},
		{
			name:           "duplicate key",
			namespace:      "test-namespace",
			key:            "test-key-1",
			expiration:     ptr(time.Now().UTC().Add(1 * time.Hour)),
			expectedStatus: "DUPLICATE",
			expectError:    false,
		},
		{
			name:           "non-expiring key",
			namespace:      "test-namespace",
			key:            "test-key-3",
			expiration:     nil,
			expectedStatus: "SUCCEEDED",
			expectError:    false,
		},
		{
			name:           "expired key",
			namespace:      "test-namespace",
			key:            "test-key-4",
			expiration:     ptr(time.Now().UTC().Add(-1 * time.Hour)),
			expectedStatus: "SUCCEEDED",
			expectError:    false,
		},
		{
			name:           "empty namespace",
			namespace:      "",
			key:            "test-key-5",
			expiration:     ptr(time.Now().UTC().Add(1 * time.Hour)),
			expectedStatus: "FAILED",
			expectError:    true,
		},
	}

	// Test with both MySQL and PostgreSQL
	for storeType, db := range dbs {
		t.Run(string(storeType), func(t *testing.T) {
			// Create factory and service
			factory, err := CreateStoreFactory(StoreFactoryConfig{
				Type: storeType,
				DB:   db,
			})
			require.NoError(t, err)

			service, err := NewIdempotencyService(factory)
			require.NoError(t, err)

			ctx := context.Background()

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// First attempt
					status1, err1 := service.SetKey(ctx, tc.namespace, tc.key, tc.expiration)
					if tc.expectError {
						assert.Error(t, err1)
					} else {
						assert.NoError(t, err1)
						assert.Equal(t, tc.expectedStatus, status1)
					}

					if !tc.expectError {
						// Verify key exists in store
						key, err := service.store.Get(ctx, tc.namespace, tc.key)
						assert.NoError(t, err)
						if tc.expiration != nil && tc.expiration.Before(now) {
							assert.Nil(t, key, "Expired key should not be returned")
						} else {
							assert.NotNil(t, key)
							if key != nil {
								assert.Equal(t, tc.namespace, key.Namespace)
								assert.Equal(t, tc.key, key.Key)
								if tc.expiration != nil {
									// Convert stored time to UTC before comparison
									storedTime := key.ExpiresAt.UTC()
									expectedTime := tc.expiration.UTC()
									assert.WithinDuration(t, expectedTime.UTC(), storedTime.UTC(), time.Second)
								} else {
									assert.Nil(t, key.ExpiresAt)
								}
							}
						}
					}
				})
			}
		})
	}
}

func TestConcurrentSetKeyE2E(t *testing.T) {
	dbs, cleanup := setupTestDatabases(t)
	defer cleanup()

	for storeType, db := range dbs {
		t.Run(string(storeType), func(t *testing.T) {
			factory, err := CreateStoreFactory(StoreFactoryConfig{
				Type: storeType,
				DB:   db,
			})
			require.NoError(t, err)

			service, err := NewIdempotencyService(factory)
			require.NoError(t, err)

			ctx := context.Background()
			namespace := "concurrent-test"
			key := "concurrent-key"
			expiration := time.Now().UTC().Add(1 * time.Hour)

			// Launch concurrent SetKey operations
			concurrency := 10
			results := make(chan string, concurrency)
			errors := make(chan error, concurrency)

			for i := 0; i < concurrency; i++ {
				go func() {
					status, err := service.SetKey(ctx, namespace, key, &expiration)
					results <- status
					errors <- err
				}()
			}

			// Collect and verify results
			successCount := 0
			duplicateCount := 0
			for i := 0; i < concurrency; i++ {
				err := <-errors
				require.NoError(t, err)

				status := <-results
				switch status {
				case "SUCCEEDED":
					successCount++
				case "DUPLICATE":
					duplicateCount++
				}
			}

			// Verify only one operation succeeded
			assert.Equal(t, 1, successCount)
			assert.Equal(t, concurrency-1, duplicateCount)

			// Verify final state
			generatedKey, err := service.store.Get(ctx, namespace, key)
			require.NoError(t, err)
			require.NotNil(t, generatedKey)
			assert.Equal(t, namespace, generatedKey.Namespace)
			assert.WithinDuration(t, expiration, *generatedKey.ExpiresAt, time.Second)
		})
	}
}

// Helper function to create pointer to time.Time
func ptr(t time.Time) *time.Time {
	return &t
}
