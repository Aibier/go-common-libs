package configurablewindow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func (dw *durationWindow) increment(ctx context.Context, pipeline redis.Pipeliner) error {
	now := time.Now().UnixNano()
	score := float64(now)
	member := fmt.Sprintf("%d", now)
	return pipeline.ZAdd(ctx, dw.keyPrefix, redis.Z{Score: score, Member: member}).Err()
}

func (dw *durationWindow) removeExpired(ctx context.Context, pipeline redis.Pipeliner) error {
	now := time.Now().UnixNano()
	minScore := float64(now - dw.duration.Nanoseconds())
	return pipeline.ZRemRangeByScore(ctx, dw.keyPrefix, "0", fmt.Sprintf("%.0f", minScore)).Err()
}

func (dw *durationWindow) countRequests(ctx context.Context) (int64, error) {
	now := time.Now().UnixNano()
	minScore := float64(now - dw.duration.Nanoseconds())
	return dw.redisClient.ZCount(ctx, dw.keyPrefix, fmt.Sprintf("%.0f", minScore), "+inf").Result()
}

func (rl *ConfigurableWindowRateLimiter) CheckVersion(version int) bool {
	return rl.durationWindow.configVersion == version
}

func (rl *ConfigurableWindowRateLimiter) Allow(ctx context.Context) (bool, error) {
	pipeline := rl.durationWindow.redisClient.TxPipeline()

	err := rl.durationWindow.increment(ctx, pipeline)
	if err != nil {
		return false, fmt.Errorf("error incrementing sliding window: %w", err)
	}

	err = rl.durationWindow.removeExpired(ctx, pipeline)
	if err != nil {
		return false, fmt.Errorf("error removing expired sliding window entries: %w", err)
	}

	_, err = pipeline.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("error executing Redis pipeline: %w", err)
	}

	count, err := rl.durationWindow.countRequests(ctx)
	if err != nil {
		return false, fmt.Errorf("error counting sliding window requests: %w", err)
	}

	allowedRequests := int64(rl.durationWindow.rate * rl.durationWindow.duration.Seconds())
	return count <= allowedRequests, nil
}

type LimiterConfig struct {
	Rate   float64 `json:"rate"`
	Window int     `json:"window"`
}

func getConfig(cfgStr string) LimiterConfig {
	var lc LimiterConfig
	if err := json.Unmarshal([]byte(cfgStr), &lc); err != nil {
		return LimiterConfig{
			Rate:   -1,
			Window: -1,
		}
	}
	return lc
}

func NewConfigurableWindowRateLimiter(rc *redis.Client, keyPrefix string, config string) *ConfigurableWindowRateLimiter {
	lc := getConfig(config)
	if lc.Rate < 0.0 || lc.Window < 0 {
		return nil
	}
	var rate float64 = lc.Rate
	var duration time.Duration = time.Duration(lc.Window) * time.Second
	return &ConfigurableWindowRateLimiter{
		durationWindow: NewDurationWindow(rc, keyPrefix, rate, duration),
	}
}
