package tokenbucket_lua

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBucketLua manages the rate limiting logic.
type TokenBucketLua struct {
	redis         *redis.Client // Redis client to interact with the Redis server
	name          string        // Name of the token bucket
	maxLocks      int           // Maximum number of tokens allowed in the bucket
	decay         int           // Time in seconds for the tokens to decay
	decaysAt      int64         // Timestamp when the tokens will decay
	remaining     int           // Number of remaining tokens in the bucket
	configVersion int           // Version of the configuration
}

// NewTokenBucketLua creates a new TokenBucketLua instance.
func NewTokenBucketLua(redis *redis.Client, name string, maxLocks, decay int) *TokenBucketLua {
	return &TokenBucketLua{
		redis:    redis,
		name:     name,
		maxLocks: maxLocks,
		decay:    decay,
	}
}

// Block attempts to acquire the lock for the given number of seconds.
func (tb *TokenBucketLua) Block(ctx context.Context, timeout time.Duration, sleep time.Duration) (bool, error) {
	starting := time.Now()

	for !tb.Acquire(ctx) {
		if time.Since(starting) >= timeout {
			return false, nil
		}

		time.Sleep(sleep)
	}

	return true, nil
}

// Acquire attempts to acquire the lock.
func (tb *TokenBucketLua) Acquire(ctx context.Context) bool {
	results, err := acquireLockLuaScript.Run(ctx, tb.redis, []string{tb.name}, time.Now().UnixNano(), tb.decay, tb.maxLocks).Result()
	if err != nil {
		print(err.Error())
		return false
	}

	res := results.([]interface{})
	if res[0] == nil || res[1] == nil || res[2] == nil {
		return false
	}
	tb.decaysAt = res[1].(int64)
	tb.remaining = int(res[2].(int64))

	return res[0].(int64) == 1
}

// TooManyAttempts determines if the key has been accessed too many times.
func (tb *TokenBucketLua) TooManyAttempts(ctx context.Context) bool {
	results, err := tooManyAttemptsLuaScript.Run(ctx, tb.redis, []string{tb.name}, time.Now().UnixNano(), tb.decay, tb.maxLocks).Result()
	if err != nil {
		return false
	}

	res := results.([]interface{})
	if res[0] == nil || res[1] == nil {
		return false
	}
	tb.decaysAt = res[0].(int64)
	tb.remaining = int(res[1].(int64))
	return tb.remaining <= 0
}

// Clear clears the limiter.
func (tb *TokenBucketLua) Clear(ctx context.Context) {
	tb.redis.Del(ctx, tb.name)
}

/** acquireLockLuaScript defines the Lua script for acquiring a lock.
*
* KEYS[1] - Name of the token bucket
* ARGV[1] - Current time in nanoseconds
* ARGV[2] - Time in seconds for the tokens to decay
* ARGV[3] - Maximum number of tokens allowed in the bucket
*
* Returns: {success, end, remaining}
 */
var acquireLockLuaScript = redis.NewScript(`
local function reset()
    -- Set the start time, end time in nanoseconds and count
    local start_time = tonumber(ARGV[1])
    local end_time = start_time + tonumber(ARGV[2])
    redis.call('HMSET', KEYS[1], 'start', start_time, 'end', end_time, 'count', 1)
    -- Set the expiration time for the key
    return redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]) / 1e9 * 2)
end

-- If the key does not exist, reset it and return the initial values
if redis.call('EXISTS', KEYS[1]) == 0 then
    return {reset(), tonumber(ARGV[1]) + tonumber(ARGV[2]), tonumber(ARGV[3]) - 1}
end

-- Check if the current time in nanoseconds is within the valid time window
if tonumber(ARGV[1]) >= tonumber(redis.call('HGET', KEYS[1], 'start')) and tonumber(ARGV[1]) <= tonumber(redis.call('HGET', KEYS[1], 'end')) then
    -- Increment the count and calculate the remaining locks
    local currentCount = tonumber(redis.call('HINCRBY', KEYS[1], 'count', 1))
    local remaining = tonumber(ARGV[3]) - currentCount
    return {
        currentCount <= tonumber(ARGV[3]),
        tonumber(redis.call('HGET', KEYS[1], 'end')),
        remaining
    }
end

-- If the current time is outside the valid time window, reset the key
return {reset(), tonumber(ARGV[1]) + tonumber(ARGV[2]), tonumber(ARGV[3]) - 1}
`)

/* tooManyAttemptsLuaScript defines the Lua script to determine if the key has been accessed too many times.
*
* KEYS[1] - Name of the token bucket
* ARGV[1] - Current time in nanoseconds
* ARGV[2] - Time in seconds for the tokens to decay
* ARGV[3] - Maximum number of tokens allowed in the bucket
*
* Returns: {end, remaining}
 */
var tooManyAttemptsLuaScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
    return {tonumber(ARGV[1]) + tonumber(ARGV[2]), tonumber(ARGV[3])}
end

if tonumber(ARGV[1]) >= tonumber(redis.call('HGET', KEYS[1], 'start')) and tonumber(ARGV[1]) <= tonumber(redis.call('HGET', KEYS[1], 'end')) then
    return {
        tonumber(redis.call('HGET', KEYS[1], 'end')),
        tonumber(ARGV[3]) - tonumber(redis.call('HGET', KEYS[1], 'count'))
    }
end

return {tonumber(ARGV[1]) + tonumber(ARGV[2]), tonumber(ARGV[3])}
`)
