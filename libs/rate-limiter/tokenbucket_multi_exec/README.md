## Usage of Token Bucket (MULTI-EXEC) Rate Limiter

The Token Bucket (multi-exec) rate limiter is used to control the rate of requests to a resource. Incoming requests can be allowed at any rate as long as there are tokens in the bucket. Once the tokens are exhausted, further requests are denied until tokens are refilled. The rate attribute controls the rate of refill of tokens into the bucket. It uses Redis' MULTI-EXEC commands for atomicity of updates. One key thing to note for this limiter is that both read and update don't occur atomically but read and update are both atomic individually. So, there will be spillover at high concurrency. **It's recommended to use the Lua version of the rate limiter for atomicity across read and update.**

### Example Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/go-redis/redis/v8"
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
        log.Fatalf("Failed to connect to Redis: %v", err)
    }

    // Initialize the rate limiter factory
    redisFactory := ratelimiter.NewRateLimiterFactory(redisClient, ratelimiter.TokenBucketRT)

    // Set up the configuration for the Token Bucket rate limiter
    key := "example-token-bucket"
    rate := 1
    capacity := 10
    tbConfig := ratelimiter.VersionedConfig{
        Config:  fmt.Sprintf("{\"rate\": %d, \"capacity\": %d}", rate, capacity),
        Version: 1,
    }
    val, err := json.Marshal(tbConfig)
    if err != nil {
        log.Fatalf("Failed to marshal test config for TokenBucket: %v", err)
    }
    err = redisClient.Set(ctx, "config_"+key, val, 20*time.Second).Err()
    if err != nil {
        log.Fatalf("Failed to set config for TokenBucket: %v", err)
    }

    // Get a rate limiter for the specific key
    limiter, err := redisFactory.GetRateLimiter(ctx, key)
    if err != nil {
        log.Fatalf("Failed to get rate limiter: %v", err)
    }

    // Simulate requests
    for i := 0; i < 20; i++ {
        allow, err := limiter.Allow(ctx)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        if allow {
            fmt.Println("Request allowed")
        } else {
            fmt.Println("Request denied")
        }
        time.Sleep(100 * time.Millisecond)
    }
}
```

### Example Output
Out of 15 requests, 11 passed (10 initial capacity + 1 token refilled after 10 iterations i.e. after ~1s), 4 denied

```plaintext
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request allowed
Request denied
Request denied
Request denied
Request denied
```
