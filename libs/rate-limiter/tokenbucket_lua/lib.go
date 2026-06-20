package tokenbucket_lua

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiterConfig is the configuration for a TokenBucketLuaRateLimiter.
type RateLimiterConfig struct {
	MaxLocks int           `json:"maxLocks"`
	Decay    int           `json:"decay"`
	Timeout  time.Duration `json:"timeout"`
	Sleep    time.Duration `json:"sleep"`
}

func getConfig(cfgStr string) RateLimiterConfig {
	var config RateLimiterConfig
	if err := json.Unmarshal([]byte(cfgStr), &config); err != nil {
		return RateLimiterConfig{
			MaxLocks: -1,
			Decay:    -1,
		}
	}
	return config
}

// TokenBucketLuaRateLimiter is a rate limiter that uses a token bucket algorithm and Redis atomic transactions using Lua script.
type TokenBucketLuaRateLimiter struct {
	tokenBucket *TokenBucketLua
	timeout     time.Duration
	sleep       time.Duration
}

// NewTokenBucketLuaRateLimiter creates a new TokenBucketLuaRateLimiter with the given rate and capacity.
func NewTokenBucketLuaRateLimiter(redisClient *redis.Client, keyPrefix string, config string) *TokenBucketLuaRateLimiter {
	rlConfig := getConfig(config)
	if rlConfig.MaxLocks < 0.0 || rlConfig.Decay < 0.0 {
		return nil
	}

	return &TokenBucketLuaRateLimiter{
		tokenBucket: NewTokenBucketLua(redisClient, keyPrefix, rlConfig.MaxLocks, rlConfig.Decay),
		timeout:     rlConfig.Timeout,
		sleep:       rlConfig.Sleep,
	}
}

// Allow executes the given callback if a lock is obtained, otherwise calls the failure callback.
func (b *TokenBucketLuaRateLimiter) Allow(ctx context.Context) (bool, error) {
	result, err := b.tokenBucket.Block(ctx, b.timeout, b.sleep)
	if err != nil {
		return false, err
	}
	return result, nil
}

// Clear clears the token bucket.
func (b *TokenBucketLuaRateLimiter) Clear(ctx context.Context) {
	b.tokenBucket.Clear(ctx)
}

func (rl *TokenBucketLuaRateLimiter) CheckVersion(version int) bool {
	return rl.tokenBucket.configVersion == version
}
