package idempotency

import (
	"context"
	"idempotency/common"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	data sync.Map // namespace -> map[string]*Key
	mu   sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{}
}

func (m *mockStore) Set(ctx context.Context, namespace string, key string, expiration *time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create namespace map
	var nsMap map[string]*Key
	if v, ok := m.data.Load(namespace); ok {
		nsMap = v.(map[string]*Key)
	} else {
		nsMap = make(map[string]*Key)
		m.data.Store(namespace, nsMap)
	}

	// Check for existing key
	if _, exists := nsMap[key]; exists {
		return common.DUPLICATE, nil
	}

	// Store new key
	nsMap[key] = &Key{
		Namespace: namespace,
		Key:       key,
		ExpiresAt: expiration,
	}

	return common.SUCCEEDED, nil
}

func (m *mockStore) Get(ctx context.Context, namespace string, key string) (*Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get namespace map
	if nsMapInterface, ok := m.data.Load(namespace); ok {
		nsMap := nsMapInterface.(map[string]*Key)
		if key, exists := nsMap[key]; exists {
			if key.ExpiresAt != nil && time.Now().Before(*key.ExpiresAt) {
				return key, nil
			}
			// Key has expired, delete it
			delete(nsMap, key.Key)
		}
	}
	return nil, nil
}

type mockStoreFactory struct {
	store Store
}

func (f *mockStoreFactory) CreateStore() (Store, error) {
	return f.store, nil
}

func newMockStoreFactory() StoreFactory {
	return &mockStoreFactory{
		store: newMockStore(),
	}
}

func TestNewIdempotencyService(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		factory := newMockStoreFactory()
		service, err := NewIdempotencyService(factory)

		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.store)
	})

	t.Run("nil factory", func(t *testing.T) {
		service, err := NewIdempotencyService(nil)

		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "store factory cannot be nil")
	})

	t.Run("factory returns nil store", func(t *testing.T) {
		factory := &mockStoreFactory{store: nil}
		service, err := NewIdempotencyService(factory)

		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "created store cannot be nil")
	})
}

func TestIdempotencyService_SetKey(t *testing.T) {
	ctx := context.Background()
	factory := newMockStoreFactory()
	service, err := NewIdempotencyService(factory)
	require.NoError(t, err)

	t.Run("successful set", func(t *testing.T) {
		expiration := time.Now().Add(time.Hour)
		status, err := service.SetKey(ctx, "test-namespace", "test-key", &expiration)

		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status)

		// Verify key was set
		key, err := service.store.Get(ctx, "test-namespace", "test-key")
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("empty namespace", func(t *testing.T) {
		expiration := time.Now().Add(time.Hour)
		status, err := service.SetKey(ctx, "", "test-key", &expiration)

		assert.Error(t, err)
		assert.Equal(t, common.FAILED, status)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("empty key", func(t *testing.T) {
		expiration := time.Now().Add(time.Hour)
		status, err := service.SetKey(ctx, "test-namespace", "", &expiration)

		assert.Error(t, err)
		assert.Equal(t, common.FAILED, status)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("duplicate key", func(t *testing.T) {
		expiration := time.Now().Add(time.Hour)

		// Set initial key
		status1, err := service.SetKey(ctx, "test-namespace", "duplicate-key", &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status1)

		// Try to set same key again
		status2, err := service.SetKey(ctx, "test-namespace", "duplicate-key", &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.DUPLICATE, status2)
	})

	t.Run("non-expiring key", func(t *testing.T) {
		status, err := service.SetKey(ctx, "test-namespace", "non-expiring-key", nil)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status)
	})
}

func TestIdempotencyService_Concurrent(t *testing.T) {
	ctx := context.Background()
	factory := newMockStoreFactory()
	service, err := NewIdempotencyService(factory)
	require.NoError(t, err)

	t.Run("concurrent set operations", func(t *testing.T) {
		const concurrency = 10
		results := make(chan string, concurrency)
		errors := make(chan error, concurrency)
		expiration := time.Now().Add(time.Hour)

		// Launch concurrent operations
		for i := 0; i < concurrency; i++ {
			go func() {
				status, err := service.SetKey(ctx, "test-namespace", "concurrent-key", &expiration)
				results <- status
				errors <- err
			}()
		}

		// Collect results
		successCount := 0
		for i := 0; i < concurrency; i++ {
			err := <-errors
			require.NoError(t, err)

			if <-results == common.SUCCEEDED {
				successCount++
			}
		}

		// Only one operation should succeed
		assert.Equal(t, 1, successCount)

		// Verify final state
		key, err := service.store.Get(ctx, "test-namespace", "concurrent-key")
		require.NoError(t, err)
		assert.NotNil(t, key)
	})
}

func TestIdempotencyService_MultipleNamespaces(t *testing.T) {
	ctx := context.Background()
	factory := newMockStoreFactory()
	service, err := NewIdempotencyService(factory)
	require.NoError(t, err)

	t.Run("same key different namespaces", func(t *testing.T) {
		expiration := time.Now().Add(time.Hour)

		// Set key in first namespace
		status1, err := service.SetKey(ctx, "namespace1", "same-key", &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status1)

		// Set same key in second namespace
		status2, err := service.SetKey(ctx, "namespace2", "same-key", &expiration)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCEEDED, status2)

		// Verify both keys exist
		key1, err := service.store.Get(ctx, "namespace1", "same-key")
		require.NoError(t, err)
		assert.NotNil(t, key1)

		key2, err := service.store.Get(ctx, "namespace2", "same-key")
		require.NoError(t, err)
		assert.NotNil(t, key2)
	})
}

func TestIdempotencyService_Validation(t *testing.T) {
	ctx := context.Background()
	factory := newMockStoreFactory()
	service, err := NewIdempotencyService(factory)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		namespace  string
		key        string
		expiration *time.Time
		wantStatus string
		wantErr    string
	}{
		{
			name:       "blank namespace and key",
			namespace:  "",
			key:        "",
			expiration: nil,
			wantStatus: common.FAILED,
			wantErr:    "validation failed",
		},
		{
			name:       "blank namespace only",
			namespace:  "",
			key:        "valid-key",
			expiration: nil,
			wantStatus: common.FAILED,
			wantErr:    "validation failed",
		},
		{
			name:       "blank key only",
			namespace:  "valid-namespace",
			key:        "",
			expiration: nil,
			wantStatus: common.FAILED,
			wantErr:    "validation failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := service.SetKey(ctx, tc.namespace, tc.key, tc.expiration)

			assert.Equal(t, tc.wantStatus, status)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
