## Use of Token Bucket (Lua) Rate Limiter
The Token Bucket (Lua) rate limiter is another implementation to control the rate of requests to a resource using Lua scripting in Redis. This implementation can be more efficient in certain scenarios due to the atomic nature of Lua scripts in Redis.

### Example Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/Aibier/go-common-libs/ratelimiter"
)

func main() {
    // Initialize Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    // Ping the Redis server to check if it's connected properly
    ctx := context.Background()
    if err := redisClient.Ping(ctx).Err(); err != nil {
        t.Fatalf("Failed to connect to Redis: %v", err)
    }

    // Initialize the rate limiter factory
    redisFactory := NewRateLimiterFactory(redisClient, TokenBucketLuaRT)

    // Set up the configuration for the Token Bucket (Lua) rate limiter
    /*
    *   Example token bucket redis key for a rate-limited provider API endpoint. 
    *   Format: {service}:{module/feature}:{action}. 
    *   In case two services share the same token bucket 
    *   for a common action which should be globally rate-limited, 
    *   please use a common prefix like "shared" instead of service name.
    */
    key := "payout-service:dbs-hk:post-v1-transfers"  
    
    var maxLocks = 3                        // Maximum number of locks/tokens allowed in the bucket
	var decay = 1 * time.Second             // Time in seconds for the token bucket to decay/expire
	var timeout = 500 * time.Millisecond    // Timeout for acquiring a token/lock from the bucket
	var sleep = 100 * time.Millisecond      // Sleep duration between retries for acquiring a token/lock

	var tbConfig = VersionedConfig{
		Config:  fmt.Sprintf("{\"maxLocks\": %d, \"decay\": %d, \"timeout\": %d, \"sleep\": %d}", maxLocks, decay, timeout, sleep),
		Version: 1,
	}
    val, err := json.Marshal(tbConfig)
    if err != nil {
        t.Fatalf("Failed to marshal test config for TokenBucketLua: %v", err)
    }
	redisClient.Set(ctx, "config_"+key, val, 20*time.Second).Err()

    // Create the rate limiter
    limiter, err := redisFactory.GetRateLimiter(ctx, key)
    if err != nil {
        t.Fatalf("Failed to create rate limiter: %v", err)
    }

    // Simulate requests
    for i := 0; i < 20; i++ {
        allowed, err := limiter.Allow(ctx)
        if err != nil {
            t.Fatalf("Failed to acquire token: %v", err)
        }
        if allowed {
            fmt.Printf("Request %d: Allowed\n", i+1)
        } else {
            fmt.Printf("Request %d: Denied\n", i+1)
        }
        time.Sleep(100 * time.Millisecond) // Sleep for 100ms between requests
    }
}
```

### Example Output
As expected, one request is denied for every three allowed requests. It's left as an assignment for the lib user to understand why such a behavior. Hint: Look at the timeout and sleep params set for the token bucket above and the sleep time set in the simulation for loop.

```plaintext
Request 1: Allowed
Request 2: Allowed
Request 3: Allowed
Request 4: Denied
Request 5: Allowed
Request 6: Allowed
Request 7: Allowed
Request 8: Denied
Request 9: Allowed
Request 10: Allowed
Request 11: Allowed
Request 12: Denied
Request 13: Allowed
Request 14: Allowed
Request 15: Allowed
Request 16: Denied
Request 17: Allowed
Request 18: Allowed
Request 19: Allowed
Request 20: Denied
```