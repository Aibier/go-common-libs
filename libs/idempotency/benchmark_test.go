package idempotency

import (
	"context"
	"database/sql"
	"fmt"
	"idempotency/common"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func BenchmarkIdempotencyService(b *testing.B) {
	// Setup test databases
	mysqlDB, err := sql.Open("mysql", "root:@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true")
	require.NoError(b, err)
	defer mysqlDB.Close()

	postgresDB, err := sql.Open("postgres", "postgres://postgres:@localhost:5432/testdb?sslmode=disable")
	require.NoError(b, err)
	defer postgresDB.Close()

	// Create stores
	stores := map[string]Store{
		"MySQL":    setupMySQLStore(b, mysqlDB),
		"Postgres": setupPostgresStore(b, postgresDB),
		"Mock":     newMockStore(),
	}

	// Run benchmarks for each store type
	for storeName, store := range stores {
		b.Run(storeName, func(b *testing.B) {
			benchmarkStore(b, store)
		})
	}
}

func setupMySQLStore(b *testing.B, db *sql.DB) Store {
	store, err := NewMySQLStore(db)
	require.NoError(b, err)
	return store
}

func setupPostgresStore(b *testing.B, db *sql.DB) Store {
	store, err := NewPostgresStore(db)
	require.NoError(b, err)
	return store
}

func benchmarkStore(b *testing.B, store Store) {
	ctx := context.Background()

	// Benchmark SetKey
	b.Run("SetKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			expiration := time.Now().Add(time.Hour)
			_, err := store.Set(ctx, "benchmark", key, &expiration)
			require.NoError(b, err)
		}
	})

	// Benchmark GetKey
	b.Run("GetKey", func(b *testing.B) {
		// Setup: Insert a key first
		key := "benchmark-get-key"
		expiration := time.Now().Add(time.Hour)
		_, err := store.Set(ctx, "benchmark", key, &expiration)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.Get(ctx, "benchmark", key)
			require.NoError(b, err)
		}
	})

	// Benchmark Concurrent SetKey
	b.Run("ConcurrentSetKey", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent-key-%d", i)
				expiration := time.Now().Add(time.Hour)
				_, err := store.Set(ctx, "benchmark", key, &expiration)
				require.NoError(b, err)
				i++
			}
		})
	})
}

func BenchmarkGenerator(b *testing.B) {
	generator := NewGenerator()

	// Benchmark ID Generation
	b.Run("GenerateID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := generator.GenerateID()
			require.NoError(b, err)
		}
	})

	// Benchmark Concurrent ID Generation
	b.Run("ConcurrentGenerateID", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := generator.GenerateID()
				require.NoError(b, err)
			}
		})
	})
}

func BenchmarkIdempotencyService_E2E(b *testing.B) {
	// Setup configurations for different store types
	configs := []struct {
		name      string
		storeType StoreType
		db        *sql.DB
	}{
		{
			name:      "MySQL",
			storeType: StoreTypeMySQL,
			db:        setupMySQLDB(b),
		},
		{
			name:      "Postgres",
			storeType: StoreTypePostgres,
			db:        setupPostgresDB(b),
		},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			factory, err := CreateStoreFactory(StoreFactoryConfig{
				Type: cfg.storeType,
				DB:   cfg.db,
			})
			require.NoError(b, err)

			service, err := NewIdempotencyService(factory)
			require.NoError(b, err)

			ctx := context.Background()
			generator := NewGenerator()

			// Benchmark complete flow
			b.Run("CompleteFlow", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					// Generate ID
					id, err := generator.GenerateID()
					require.NoError(b, err)

					// Set expiration
					expiration := time.Now().Add(time.Hour)

					// Set key
					_, err = service.SetKey(ctx, "benchmark", id, &expiration)
					require.NoError(b, err)
				}
			})

			// Benchmark concurrent complete flow
			b.Run("ConcurrentCompleteFlow", func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						// Generate ID
						id, err := generator.GenerateID()
						require.NoError(b, err)

						// Set expiration
						expiration := time.Now().Add(time.Hour)

						// Set key
						_, err = service.SetKey(ctx, "benchmark", id, &expiration)
						require.NoError(b, err)
					}
				})
			})

			// Benchmark duplicate handling
			b.Run("DuplicateHandling", func(b *testing.B) {
				// Setup: Create initial key
				id, err := generator.GenerateID()
				require.NoError(b, err)
				expiration := time.Now().Add(time.Hour)
				_, err = service.SetKey(ctx, "benchmark", id, &expiration)
				require.NoError(b, err)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err = service.SetKey(ctx, "benchmark", id, &expiration)
					require.NoError(b, err)
				}
			})
		})
	}
}

func BenchmarkValidator(b *testing.B) {
	service, err := NewIdempotencyService(newMockStoreFactory())
	require.NoError(b, err)

	validKey := Key{
		Namespace: "benchmark",
		Key:       "test-key",
		ExpiresAt: ptr(time.Now().Add(time.Hour)),
	}

	invalidKey := Key{
		Namespace: "",
		Key:       "",
	}

	b.Run("ValidateValidKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := service.validator.Struct(validKey)
			require.NoError(b, err)
		}
	})

	b.Run("ValidateInvalidKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = service.validator.Struct(invalidKey)
		}
	})

	b.Run("ConcurrentValidation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := service.validator.Struct(validKey)
				require.NoError(b, err)
			}
		})
	})
}

func BenchmarkRetryMechanism(b *testing.B) {
	ctx := context.Background()
	config := common.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond * 10,
		Factor:       2.0,
	}

	b.Run("SuccessFirstTry", func(b *testing.B) {
		operation := func() error {
			return nil
		}

		for i := 0; i < b.N; i++ {
			err := common.ExecuteWithRetry(ctx, config, operation)
			require.NoError(b, err)
		}
	})

	b.Run("SuccessAfterRetry", func(b *testing.B) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 2 {
				return fmt.Errorf("temporary error")
			}
			return nil
		}

		for i := 0; i < b.N; i++ {
			attempts = 0
			err := common.ExecuteWithRetry(ctx, config, operation)
			require.NoError(b, err)
		}
	})

	b.Run("ConcurrentRetries", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			operation := func() error {
				return nil
			}

			for pb.Next() {
				err := common.ExecuteWithRetry(ctx, config, operation)
				require.NoError(b, err)
			}
		})
	})
}

// Helper functions
func setupMySQLDB(b *testing.B) *sql.DB {
	db, err := sql.Open("mysql", "root:@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true")
	require.NoError(b, err)
	return db
}

func setupPostgresDB(b *testing.B) *sql.DB {
	db, err := sql.Open("postgres", "postgres://postgres:@localhost:5432/testdb?sslmode=disable")
	require.NoError(b, err)
	return db
}
