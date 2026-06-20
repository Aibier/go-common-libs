package idempotency

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// StoreType represents the type of storage backend
type StoreType string

const (
	StoreTypeMySQL    StoreType = "mysql"
	StoreTypePostgres StoreType = "postgres"
)

type Store interface {
	Set(ctx context.Context, namespace string, key string, expiration *time.Time) (string, error)
	Get(ctx context.Context, namespace string, key string) (*Key, error)
}

// StoreFactory interface for creating stores
type StoreFactory interface {
	CreateStore() (Store, error)
}

// StoreFactoryConfig holds configuration for creating store factories
type StoreFactoryConfig struct {
	Type   StoreType
	DB     *sql.DB
	Prefix string
}

// CreateStoreFactory creates the appropriate store factory based on configuration
func CreateStoreFactory(config StoreFactoryConfig) (StoreFactory, error) {
	if config.Type == "" {
		return nil, fmt.Errorf("store type is required")
	}

	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	switch config.Type {
	case StoreTypeMySQL:
		return NewMySQLStoreFactory(config.DB), nil
	case StoreTypePostgres:
		return NewPostgresStoreFactory(config.DB), nil
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.Type)
	}
}
