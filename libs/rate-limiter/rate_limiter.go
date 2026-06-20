package ratelimiter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	cw "github.com/Aibier/go-common-libs/libs/rate-limiter/configurablewindow"
	tbl "github.com/Aibier/go-common-libs/libs/rate-limiter/tokenbucket_lua"
	tbme "github.com/Aibier/go-common-libs/libs/rate-limiter/tokenbucket_multi_exec"
)

const (
	ConfigurableWindowRT   = "ConfigurableWindow"
	TokenBucketMultiExecRT = "TokenBucketMultiExec"
	TokenBucketLuaRT       = "TokenBucketLua"
)

type RateLimiter interface {
	Allow(ctx context.Context) (bool, error)
	CheckVersion(version int) bool
}

type Factory struct {
	redisClient *redis.Client
	limiters    map[string]RateLimiter
	rlType      string
	mu          sync.Mutex
}

/*
Eg for confgurable window config format:

	{
		"config" : \"{
			"window": 1,
			"rate": 1
		}\",
		"version": 1
	}
*/
type VersionedConfig struct {
	Config  string `json:"config"`
	Version int    `json:"version"`
}

func getVersionedConfig(ctx context.Context, rc *redis.Client, key string) VersionedConfig {
	vcStr, err := rc.Get(ctx, "config_"+key).Bytes()
	if err != nil {
		return VersionedConfig{
			Config:  "",
			Version: -1,
		}
	}
	var vc VersionedConfig
	if err = json.Unmarshal(vcStr, &vc); err != nil {
		return VersionedConfig{
			Config:  "",
			Version: -1,
		}
	}
	return vc
}

func (f *Factory) GetRateLimiter(ctx context.Context, key string, vc VersionedConfig) (RateLimiter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var limiter RateLimiter
	if limiter, exists := f.limiters[key]; exists {
		if limiter.CheckVersion(vc.Version) {
			return limiter, nil
		}
		delete(f.limiters, key)
	}

	switch f.rlType {
	case ConfigurableWindowRT:
		limiter = cw.NewConfigurableWindowRateLimiter(f.redisClient, key, vc.Config)
	case TokenBucketMultiExecRT:
		limiter = tbme.NewTokenBucketMultiExecRateLimiter(f.redisClient, key, vc.Config)
	case TokenBucketLuaRT:
		limiter = tbl.NewTokenBucketLuaRateLimiter(f.redisClient, key, vc.Config)
	default:
		return nil, fmt.Errorf("unsupported rate limiter type")
	}

	f.limiters[key] = limiter
	return limiter, nil
}

func (f *Factory) CreateOrUpdateConfig(ctx context.Context, key string, config string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	vc := getVersionedConfig(ctx, f.redisClient, key)
	version := 1
	if vc.Version > 0 {
		version = vc.Version + 1
	}
	var updateVC = VersionedConfig{
		Config:  config,
		Version: version,
	}
	val, err := json.Marshal(updateVC)
	if err != nil {
		return err
	}
	return f.redisClient.Set(ctx, "config_"+key, val, 0).Err()
}

func NewRateLimiterFactory(redisClient *redis.Client, rlType string) *Factory {
	return &Factory{
		redisClient: redisClient,
		limiters:    make(map[string]RateLimiter),
		rlType:      rlType,
	}
}
