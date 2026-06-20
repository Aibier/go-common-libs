package idempotency

import (
	"context"
	"database/sql"
	"fmt"
	"idempotency/common"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	mysqlCreateTableSQL = `
        CREATE TABLE IF NOT EXISTS idempotency_keys (
            namespace VARCHAR(255) NOT NULL,
            idempotency_key VARCHAR(255) NOT NULL,
            expires_at DATETIME(6) NULL,
            created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6),
			deleted_at DATETIME(6) NULL,
            PRIMARY KEY(idempotency_key, namespace),
			INDEX idx_expiration (expires_at),
			INDEX idx_namespace_key_expiration (idempotency_key, namespace, expires_at)
        )`

	mysqlInsertSQL = `
        INSERT INTO idempotency_keys
        (namespace, idempotency_key, expires_at)
        VALUES (?, ?, ?)`

	mysqlSelectSQL = `
        SELECT namespace, idempotency_key, expires_at
        FROM idempotency_keys
        WHERE idempotency_key = ? AND namespace = ?
        AND (expires_at IS NULL OR expires_at > UTC_TIMESTAMP())`

	mysqlDuplicateEntryCode = 1062
)

type MySQLStore struct {
	db    *sql.DB
	retry common.RetryConfig
}

func NewMySQLStoreFactory(db *sql.DB) StoreFactory {
	return &MySQLStore{
		db:    db,
		retry: common.DefaultRetryConfig,
	}
}

func (f *MySQLStore) CreateStore() (Store, error) {
	if f.db == nil {
		return nil, fmt.Errorf("mysql connection is required")
	}
	return NewMySQLStore(f.db)
}

func NewMySQLStore(db *sql.DB) (*MySQLStore, error) {
	store := &MySQLStore{
		db:    db,
		retry: common.DefaultRetryConfig,
	}

	if err := store.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize MySQL store: %w", err)
	}

	return store, nil
}

func (s *MySQLStore) initialize() error {
	// Create tables
	if _, err := s.db.Exec(mysqlCreateTableSQL); err != nil {
		return err
	}

	return nil
}

func (s *MySQLStore) Set(ctx context.Context, namespace string, key string, expiration *time.Time) (string, error) {
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
		_, err = tx.ExecContext(
			ctx,
			mysqlInsertSQL,
			namespace,
			key,
			expiration,
		)

		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == mysqlDuplicateEntryCode {
				// Return custom error for unique constraint violation
				return &common.UniqueConstraintError{Err: mysqlErr}
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

func (s *MySQLStore) Get(ctx context.Context, namespace string, key string) (*Key, error) {
	var result Key

	operation := func() error {

		err := s.db.QueryRowContext(ctx, mysqlSelectSQL, key, namespace).Scan(
			&result.Namespace,
			&result.Key,
			&result.ExpiresAt,
		)

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
