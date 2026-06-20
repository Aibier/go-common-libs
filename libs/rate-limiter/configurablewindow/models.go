package configurablewindow

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type durationWindow struct {
	redisClient   *redis.Client
	keyPrefix     string
	rate          float64
	duration      time.Duration
	configVersion int
}

func NewDurationWindow(redisClient *redis.Client, keyPrefix string, rate float64, window time.Duration) *durationWindow {
	return &durationWindow{
		redisClient: redisClient,
		keyPrefix:   keyPrefix, // keyPrefix is the key in Redis, can be formed by provider name and endpoint
		rate:        rate,      // rate is the number of requests allowed per second
		duration:    window,    // duration is the time window in which the rate is calculated in seconds
	}
}

type ConfigurableWindowRateLimiter struct {
	durationWindow *durationWindow
}
