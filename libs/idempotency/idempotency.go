package idempotency

import (
	"context"
	"fmt"
	"idempotency/common"
	"time"

	"github.com/go-playground/validator/v10"
)

type Key struct {
	Key       string     `validate:"required,min=1"` // Unique identifier
	Namespace string     `validate:"required,min=1"` // Business domain/type
	ExpiresAt *time.Time // Expiration time
}

// IdempotencyService handles idempotent operations
type IdempotencyService struct {
	store     Store
	validator *validator.Validate
}

// NewIdempotencyService creates a new service using the provided store factory
func NewIdempotencyService(factory StoreFactory) (*IdempotencyService, error) {
	if factory == nil {
		return nil, fmt.Errorf("store factory cannot be nil")
	}

	store, err := factory.CreateStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	if store == nil {
		return nil, fmt.Errorf("created store cannot be nil")
	}

	return &IdempotencyService{
		store:     store,
		validator: validator.New(),
	}, nil
}

// SetKey attempts to set an idempotency key for a given namespace with an optional expiration time.
// It ensures that only one operation with the same key in the same namespace can succeed.
//
// Parameters:
//   - ctx: Context for the operation, which can be used for cancellation
//   - namespace: The business domain or category for the operation (e.g., "payments", "orders")
//   - key: A unique identifier for the operation
//   - expiration: Optional expiration time for the key. If nil, the key never expires
//
// Returns:
//   - string: Status of the operation ("SUCCEEDED", "DUPLICATE", or "FAILED")
//   - error: Any error that occurred during the operation
//
// The function guarantees idempotency by:
//  1. First checking if the key already exists (if expiration is provided)
//  2. Attempting to set the key with proper concurrency control
//  3. Handling race conditions through database constraints
//  4. Ensuring that any future requests with the same key in the same namespace will be rejected
func (s *IdempotencyService) SetKey(ctx context.Context, namespace string, key string, expiration *time.Time) (string, error) {

	keyReq := Key{
		Namespace: namespace,
		Key:       key,
		ExpiresAt: expiration,
	}

	if err := s.validator.Struct(keyReq); err != nil {
		return common.FAILED, fmt.Errorf("validation failed: %w", err)
	}
	// Check existing entry
	// If expiration is nil, it means the key should not expire so we don't need to make a DB call
	// to look for it we will come to know when we try to set it
	if expiration != nil {
		existing, err := s.store.Get(ctx, namespace, key)
		if err != nil {
			return common.FAILED, err
		}
		if existing != nil {
			return common.DUPLICATE, nil
		}
	}

	status, err := s.store.Set(ctx, namespace, key, expiration)
	if err != nil {
		return status, err
	}

	return status, nil
}
