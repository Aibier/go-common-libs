package idempotency

import (
	"context"
	"database/sql"
	"fmt"
	"idempotency/common"
	"time"

	"github.com/lib/pq"
)

const (
	postgresCreateTableSQL = `
        CREATE TABLE IF NOT EXISTS idempotency_keys (
            namespace VARCHAR(255) NOT NULL,
            idempotency_key VARCHAR(255) NOT NULL,
            expires_at TIMESTAMP WITH TIME ZONE NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            deleted_at TIMESTAMP WITH TIME ZONE NULL,
            PRIMARY KEY(idempotency_key, namespace)
        );
        CREATE INDEX IF NOT EXISTS idx_expiration
            ON idempotency_keys (expires_at);
        CREATE INDEX IF NOT EXISTS idx_namespace_key_expiration
            ON idempotency_keys (idempotency_key, namespace, expires_at);`

	postgresInsertSQL = `
			INSERT INTO idempotency_keys
			(namespace, idempotency_key, expires_at)
			VALUES ($1, $2, $3)`

	postgresSelectSQL = `
			SELECT namespace, idempotency_key, expires_at
			FROM idempotency_keys
			WHERE idempotency_key = $1 AND namespace = $2
			AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`

	pqUniqueViolationCode = "23505"
)

type PostgresStore struct {
	db    *sql.DB
	retry common.RetryConfig
}

func NewPostgresStoreFactory(db *sql.DB) StoreFactory {
	return &PostgresStore{
		db:    db,
		retry: common.DefaultRetryConfig,
	}
}

func (f *PostgresStore) CreateStore() (Store, error) {
	if f.db == nil {
		return nil, fmt.Errorf("postgres connection is required")
	}
	return NewPostgresStore(f.db)
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	store := &PostgresStore{
		db:    db,
		retry: common.DefaultRetryConfig,
	}

	if err := store.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize Postgres store: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) initialize() error {
	_, err := s.db.Exec(postgresCreateTableSQL)
	return err
}

func (s *PostgresStore) Set(ctx context.Context, namespace string, key string, expiration *time.Time) (string, error) {

	// Convert expiration to UTC if it's not nil
	var utcExpiration *time.Time
	if expiration != nil {
		utc := expiration.UTC()
		utcExpiration = &utc
	}
	operation := func() error {
		// Start a transaction
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelRepeatableRead,
		})
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Execute insert/update
		_, err = tx.Exec(
			postgresInsertSQL,
			namespace,
			key,
			utcExpiration,
		)

		if err != nil {
			// Check for unique violation
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pqUniqueViolationCode {
				// Return custom error for unique constraint violation
				return &common.UniqueConstraintError{Err: pqErr}
			}
			return fmt.Errorf("failed to execute insert: %w", err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		return nil
	}

	err := common.ExecuteWithRetry(ctx, s.retry, operation)
	if err != nil {
		if _, ok := err.(*common.UniqueConstraintError); ok {
			return common.DUPLICATE, nil
		}
		return common.FAILED, err
	}

	return common.SUCCEEDED, nil
}

func (s *PostgresStore) Get(ctx context.Context, namespace string, key string) (*Key, error) {
	var result Key

	operation := func() error {
		var timestamp sql.NullTime
		err := s.db.QueryRowContext(ctx, postgresSelectSQL, key, namespace).Scan(
			&result.Namespace,
			&result.Key,
			&timestamp,
		)

		// Convert ExpiresAt to UTC if not nil
		if timestamp.Valid {
			utc := timestamp.Time.UTC()
			result.ExpiresAt = &utc
		}

		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to query key: %w", err)
		}

		return nil
	}

	err := common.ExecuteWithRetry(ctx, s.retry, operation)
	if err != nil {
		return nil, err
	}

	if result.Key == "" {
		return nil, nil
	}

	return &result, nil
}
