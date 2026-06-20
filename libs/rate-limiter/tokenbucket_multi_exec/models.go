package tokenbucket_multi_exec

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	TOKENS_FIELD      = "tokens"
	LAST_REFILL_FIELD = "last_refill"
)

type TokenBucketMultiExec struct {
	redisClient   *redis.Client // Redis client
	key           string        // Redis key
	rate          float64       // Rate of tokens to add per second
	capacity      float64       // Maximum number of tokens
	configVersion int           // Version of the configuration
}

// NewTokenBucketMultiExec creates a new TokenBucketMultiExec with the specified rate and capacity.
func NewTokenBucketMultiExec(redisClient *redis.Client, key string, rate float64, capacity float64) *TokenBucketMultiExec {
	return &TokenBucketMultiExec{
		redisClient: redisClient,
		key:         key,
		rate:        rate,
		capacity:    capacity,
	}
}

// Allow checks if a request is allowed based on the current number of tokens.
func (tb *TokenBucketMultiExec) Allow(ctx context.Context) (bool, error) {
	now := time.Now().UnixNano()
	allowed, err := tb.allowRequest(ctx, now)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// allowRequest uses a Redis transaction to atomically check and update the token bucket.
func (tb *TokenBucketMultiExec) allowRequest(ctx context.Context, now int64) (bool, error) {
	// Default values if keys do not exist
	tokens, lastRefill, err := tb.getTokensAndLastRefill(ctx)
	if err != nil {
		return false, err
	}

	// Calculate new tokens
	newTokens := tb.calculateNewTokens(tokens, lastRefill, now)
	if newTokens < 1 {
		return false, nil
	}

	// Decrement tokens and update last refill time
	newTokens -= 1
	err = tb.updateTokensAndLastRefill(ctx, newTokens, now)
	if err != nil {
		return false, err
	}

	return true, nil
}

// getTokensFromCmd retrieves the current number of tokens from the Redis command.
func (tb *TokenBucketMultiExec) getTokensAndLastRefill(ctx context.Context) (float64, int64, error) {
	var tokens float64
	var lastRefill int64

	_, err := tb.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		tokensCmd := pipe.HGet(ctx, tb.key, TOKENS_FIELD)
		lastRefillCmd := pipe.HGet(ctx, tb.key, LAST_REFILL_FIELD)

		pipe.Exec(ctx)

		tokens, _ = strconv.ParseFloat(tokensCmd.Val(), 64)
		lastRefill, _ = strconv.ParseInt(lastRefillCmd.Val(), 10, 64)

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return tokens, lastRefill, nil
}

// calculateNewTokens calculates the new number of tokens based on the elapsed time.
func (tb *TokenBucketMultiExec) calculateNewTokens(tokens float64, lastRefill int64, now int64) float64 {
	elapsed := now - lastRefill
	newTokens := tokens + (float64(elapsed) * tb.rate / float64(time.Second))
	if newTokens > tb.capacity {
		newTokens = tb.capacity
	}
	return newTokens
}

// updateTokensAndRefill updates the token count and last refill time in Redis.
func (tb *TokenBucketMultiExec) updateTokensAndLastRefill(ctx context.Context, newTokens float64, now int64) error {
	_, err := tb.redisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, tb.key, TOKENS_FIELD, newTokens)
		pipe.HSet(ctx, tb.key, LAST_REFILL_FIELD, now)
		pipe.Exec(ctx)
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
