package idempotency

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestGenerator_GenerateID(t *testing.T) {
	t.Run("generates valid ULID", func(t *testing.T) {
		generator := NewGenerator()
		id, err := generator.GenerateID()
		assert.NoError(t, err)

		// Verify length and format
		assert.Len(t, id, 26)
		_, err = ulid.Parse(id)
		assert.NoError(t, err)
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		generator := NewGenerator()
		ids := make(map[string]bool)

		for i := 0; i < 1000; i++ {
			id, err := generator.GenerateID()
			assert.NoError(t, err)
			assert.False(t, ids[id], "ID should be unique")
			ids[id] = true
		}
	})

	t.Run("maintains monotonicity", func(t *testing.T) {
		generator := NewGenerator()
		var lastID string

		for i := 0; i < 100; i++ {
			currentID, err := generator.GenerateID()
			assert.NoError(t, err)
			if lastID != "" {
				assert.Greater(t, currentID, lastID, "IDs should be monotonically increasing")
			}
			lastID = currentID
		}
	})

	t.Run("timestamp extraction", func(t *testing.T) {
		generator := NewGenerator()
		now := time.Now()
		id, err := generator.GenerateID()
		assert.NoError(t, err)

		parsed, err := ulid.Parse(id)
		assert.NoError(t, err)

		timestamp := time.Unix(int64(parsed.Time())/1000, 0)
		assert.WithinDuration(t, now, timestamp, time.Second)
	})
}
