package tokenbucket_multi_exec

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// RateLimiterConfig is the configuration for a TokenBucketMultiExecRateLimiter.
type RateLimiterConfig struct {
    Rate     float64 `json:"rate"`
    Capacity float64 `json:"capacity"`
}

func getConfig(cfgStr string) RateLimiterConfig {
    var config RateLimiterConfig
    if err := json.Unmarshal([]byte(cfgStr), &config); err != nil {
        return RateLimiterConfig{
            Rate:     -1,
            Capacity: -1,
        }
    }
    return config
}

// TokenBucketMultiExecRateLimiter is a rate limiter that uses a token bucket algorithm and Redis atomic transactions using MULTI/EXEC commands.
type TokenBucketMultiExecRateLimiter struct {
    tokenBucket *TokenBucketMultiExec
}

// NewTokenBucketMultiExecRateLimiter creates a new TokenBucketMultiExecRateLimiter with the given rate and capacity.
func NewTokenBucketMultiExecRateLimiter(redisClient *redis.Client, keyPrefix string, config string) *TokenBucketMultiExecRateLimiter {
    rlConfig := getConfig(config)
    if rlConfig.Rate < 0.0 || rlConfig.Capacity < 0.0 {
        return nil
    }

    return &TokenBucketMultiExecRateLimiter{
        tokenBucket: NewTokenBucketMultiExec(redisClient, keyPrefix, rlConfig.Rate, float64(rlConfig.Capacity)),
    }
}

// Allow checks if a request is allowed based on the current number of tokens.
func (rl *TokenBucketMultiExecRateLimiter) Allow(ctx context.Context) (bool, error) {
    return rl.tokenBucket.Allow(ctx)
}

func (rl *TokenBucketMultiExecRateLimiter) CheckVersion(version int) bool {
	return rl.tokenBucket.configVersion == version
}
