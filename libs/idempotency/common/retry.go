package common

import (
	"context"
	"fmt"
	"time"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Factor       float64
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     2 * time.Second,
	Factor:       2.0,
}

// Retry executes an operation with exponential backoff retry logic.
// It will retry the operation according to the configuration provided,
// backing off exponentially between attempts.
//
// Parameters:
//   - ctx: Context that can be used to cancel the retry loop
//   - config: RetryConfig containing retry parameters:
//   - MaxAttempts: Maximum number of attempts to try the operation
//   - InitialDelay: Starting delay between retries
//   - MaxDelay: Maximum delay between retries
//   - Factor: Multiplier for exponential backoff
//   - operation: The function to be retried
//
// Returns:
//   - error: The last error encountered, or nil if the operation succeeded
//
// The function will immediately return if:
//   - The operation succeeds
//   - The context is cancelled
//   - A UniqueConstraintError is encountered
//   - Maximum attempts are reached
//
// Reference doc on why timer.channel is being drained
// and why it isn't required from go 1.2:- https://go.dev/wiki/Go123Timer
func ExecuteWithRetry(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context canceled during retry: %w", err)
		}

		if err := operation(); err == nil {
			return nil
		} else if _, ok := err.(*UniqueConstraintError); ok {
			// Return the error immediately without retrying
			return err
		} else {
			lastErr = err
		}

		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate next delay with exponential backoff
		delay = calculateDelay(config, delay)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			timer.Stop()
		case <-ctx.Done():
			timer.Stop()
			select {
			case <-timer.C: // drain the channel
			default:
			}
			return fmt.Errorf("context canceled during retry: %w", ctx.Err())
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

func calculateDelay(config RetryConfig, delay time.Duration) time.Duration {

	updatedDelay := time.Duration(float64(delay) * config.Factor)
	return min(updatedDelay, config.MaxDelay)

}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
