# go-common-libs
Common libs for type conversion, logger, rate limiter, and db connections

## Installation
```shell
go get github.com/Aibier/go-common-libs
```

## Usage

In the following example, we create a rate limiter with a configurable window of 3 requests per second. We then check if the request is allowed. If the request is allowed, we print "Request allowed" to the console. If the request is denied, we print "Request denied" to the console.

```go
package main

import "github.com/Aibier/go-common-libs/ratelimiter"

func main() {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Ping the Redis server to check if it's connected properly
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize the rate limiter factory
	redisFactory := ratelimiter.NewRateLimiterFactory(redisClient, "ConfigurableWindow")
	// Get a rate limiter for the specific key
	key := "dbs-transfer"
	rate := 3
	duration := time.Second

	limiter, err := redisFactory.GetRateLimiter(key, rate, duration)
	if err != nil {
        log.Fatalf("Failed to get rate limiter: %v", err)
		return
  }
  // Create a background context
	ctx := context.Background()
	allow, err := limiter.Allow(ctx)
	if err != nil {
		log.Printf("Error checking request allowance: %v", err)
		return
	}
	if allow {
		fmt.Println("Request allowed")
	} else {
		fmt.Println("Request denied")
	}
}
```

![Test Case Output](https://github.com/Aibier/go-common-libs/blob/main/rate-limiter.png)


#### Authors:
- [Tony Aizize](https://github.com/Aibier)
