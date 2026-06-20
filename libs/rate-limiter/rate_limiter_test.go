package ratelimiter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

var (
	redisClientInstance *redis.Client
	redisClientOnce     sync.Once
)

func setupRedisClient(t *testing.T) *redis.Client {
	redisClientOnce.Do(func() {
		redisClientInstance = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		_, err := redisClientInstance.Ping(context.Background()).Result()
		if err != nil {
			fmt.Errorf("Failed to connect to Redis: %v", err)
		}
	})
	return redisClientInstance
}

func TestConfigurableWindowRateLimiter(t *testing.T) {
	ctx := context.Background()
	redisClient := setupRedisClient(t)
	key := "test-configurable-window-rate-limiter"
	var rate = 3
	var duration = 1
	var vc = VersionedConfig{
		Config:  fmt.Sprintf("{\"window\": %d, \"rate\": %d}", duration, rate),
		Version: 1,
	}
	val, err := json.Marshal(vc)
	assert.NoError(t, err, "Failed to marshal test config")
	err = redisClient.Set(ctx, "config_"+key, val, 20*time.Second).Err()
	assert.NoError(t, err, "Failed to set config")

	// Set up rate limiter factory
	factory := NewRateLimiterFactory(redisClient, ConfigurableWindowRT)

	// Clear the Redis key before running the test
	err = redisClient.Del(ctx, key).Err()
	assert.NoError(t, err, "Failed to clear Redis key")

	var wg sync.WaitGroup
	numRequests := 10
	var allowedRequests int32 = 0

	// Function to simulate a request
	simulateRequest := func() {
		defer wg.Done()
		limiter, err := factory.GetRateLimiter(ctx, key, vc)
		if err != nil {
			t.Errorf("Error in rate limiter: %v", err)
		}
		allowed, err := limiter.Allow(context.Background())
		if err != nil {
			t.Errorf("Error in rate limiter: %v", err)
		}
		if allowed {
			atomic.AddInt32(&allowedRequests, 1)
		}
	}

	const testRounds = 5

	for round := 0; round < testRounds; round++ {
		delay := 10 * time.Millisecond

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go simulateRequest()
			time.Sleep(delay) // Add slight delay to avoid immediate burst
		}

		wg.Wait()

		fmt.Printf("[Round %d/%d]: Accepted requests: %d/%d\n", round+1, testRounds, allowedRequests, numRequests)
		assert.LessOrEqual(t, int(allowedRequests), rate, "More requests allowed than expected")
		assert.NotZero(t, int(allowedRequests), "No requests were allowed")
		allowedRequests = 0

		if round < testRounds-1 {
			// Wait for next second to make sure the bucket is replenished
			time.Sleep(time.Duration(duration) * time.Second)
		}
	}
}

func TestTokenBucketMultiExecRateLimiter(t *testing.T) {
	ctx := context.Background()
	redisClient := setupRedisClient(t)
	key := "test-token-bucket-multi-exec-rate-limiter"
	var rate = 3     // Refill rate of 3 requests per second
	var capacity = 3 // Maximum capacity of 3 requests per bucket
	var tbConfig = VersionedConfig{
		Config:  fmt.Sprintf("{\"rate\": %d, \"capacity\": %d}", rate, capacity),
		Version: 1,
	}
	val, err := json.Marshal(tbConfig)
	assert.NoError(t, err, "Failed to marshal test config for TokenBucketMultiExec")
	err = redisClient.Set(ctx, "config_"+key, val, 20*time.Second).Err()
	assert.NoError(t, err, "Failed to set config for TokenBucketMultiExec")

	// Set up rate limiter factory
	factory := NewRateLimiterFactory(redisClient, TokenBucketMultiExecRT)

	// Clear the Redis key before running the test
	err = redisClient.Del(ctx, key).Err()
	assert.NoError(t, err, "Failed to clear Redis key")

	var wg sync.WaitGroup
	numRequests := 10
	var allowedRequests int32 = 0

	// Function to simulate a request
	simulateRequest := func() {
		defer wg.Done()
		limiter, err := factory.GetRateLimiter(ctx, key, tbConfig)
		if err != nil {
			t.Errorf("Error in rate limiter: %v", err)
		}
		allowed, err := limiter.Allow(context.Background())
		if err != nil {
			t.Errorf("Error in rate limiter: %v", err)
		}
		if allowed {
			atomic.AddInt32(&allowedRequests, 1)
		}
	}

	const testRounds = 5

	for round := 0; round < testRounds; round++ {
		delay := 10 * time.Millisecond

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go simulateRequest()
			time.Sleep(delay) // Add slight delay to avoid immediate burst
		}

		wg.Wait()

		fmt.Printf("[Round %d/%d]: Accepted requests: %d/%d\n", round+1, testRounds, allowedRequests, numRequests)
		assert.LessOrEqual(t, int(allowedRequests), capacity, "More requests allowed than expected")
		assert.NotZero(t, int(allowedRequests), "No requests were allowed")
		allowedRequests = 0

		if round < testRounds-1 {
			// Wait for next second to make sure the bucket is replenished
			time.Sleep(1000 * time.Millisecond)
		}
	}
}

func TestTokenBucketLuaRateLimiter(t *testing.T) {
	ctx := context.Background()
	redisClient := setupRedisClient(t)

	key := "test-token-bucket-lua-rate-limiter" // Example key for a rate-limited provider API endpoint: "dbs::v1-transfer-endpoint"

	var maxLocks = 3                     // Maximum number of locks/tokens allowed in the bucket
	var decay = 1 * time.Second          // Time in seconds for the token bucket to decay/expire
	var timeout = 500 * time.Millisecond // Timeout for acquiring a token/lock from the bucket
	var sleep = 100 * time.Millisecond   // Sleep duration between retries for acquiring a token/lock

	var tbConfig = VersionedConfig{
		Config:  fmt.Sprintf("{\"maxLocks\": %d, \"decay\": %d, \"timeout\": %d, \"sleep\": %d}", maxLocks, decay, timeout, sleep),
		Version: 1,
	}
	val, err := json.Marshal(tbConfig)
	assert.NoError(t, err, "Failed to marshal test config for TokenBucketLua")
	err = redisClient.Set(ctx, "config_"+key, val, 20*time.Second).Err()
	assert.NoError(t, err, "Failed to set config for TokenBucketLua")

	// Set up rate limiter factory
	factory := NewRateLimiterFactory(redisClient, TokenBucketLuaRT)

	// Clear the Redis key before running the test
	err = redisClient.Del(ctx, key).Err()
	assert.NoError(t, err, "Failed to clear Redis key")

	var wg sync.WaitGroup
	numRequests := 10
	var allowedRequests int32 = 0

	limiter, err := factory.GetRateLimiter(ctx, key, tbConfig)
	if err != nil {
		t.Errorf("Error in rate limiter: %v", err)
	}

	// Function to simulate a request
	simulateRequest := func() {
		defer wg.Done()
		allowed, err := limiter.Allow(ctx)
		if err != nil {
			t.Errorf("Error in rate limiter: %v", err)
		}
		if allowed {
			atomic.AddInt32(&allowedRequests, 1)
		}
	}

	const testRounds = 5

	for round := 0; round < testRounds; round++ {
		delay := 10 * time.Millisecond

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go simulateRequest()
			time.Sleep(delay) // Add slight delay to avoid immediate burst
		}

		wg.Wait()

		fmt.Printf("[Round %d/%d]: Accepted requests: %d/%d\n", round+1, testRounds, allowedRequests, numRequests)
		assert.LessOrEqual(t, int(allowedRequests), maxLocks, "More requests allowed than expected")
		assert.NotZero(t, int(allowedRequests), "No requests were allowed")
		allowedRequests = 0

		if round < testRounds-1 {
			// Wait for next second to make sure the bucket is replenished
			time.Sleep(decay)
		}
	}
}

func TestCreateConfig(t *testing.T) {
	ctx := context.Background()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Ensure Redis is working
	err := redisClient.Ping(ctx).Err()
	assert.NoError(t, err, "Failed to connect to Redis")
	key := "test-rate-limiter"
	err = redisClient.Del(ctx, "config_"+key).Err()
	assert.NoError(t, err, "Failed to clear Redis key")
	factory := NewRateLimiterFactory(redisClient, ConfigurableWindowRT)
	var rate = 3
	var duration = 1
	cfgStr := fmt.Sprintf("{\"window\": %d, \"rate\": %d}", duration, rate)

	factory.CreateOrUpdateConfig(ctx, key, cfgStr)
	vc := getVersionedConfig(ctx, redisClient, key)
	assert.Equal(t, 1, vc.Version, "Version did not match")
	assert.Equal(t, cfgStr, vc.Config, "Congfiguration did not match")

	newCfgStr := fmt.Sprintf("{\"window\": %d, \"rate\": %d}", duration, rate+5)
	factory.CreateOrUpdateConfig(ctx, key, newCfgStr)
	vc = getVersionedConfig(ctx, redisClient, key)
	assert.Equal(t, 2, vc.Version, "Version did not match")
	assert.Equal(t, newCfgStr, vc.Config, "Congfiguration did not match")
	err = redisClient.Del(ctx, "config_"+key).Err()
	assert.NoError(t, err, "Failed to clear Redis key")
}
