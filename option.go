package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type Option func(*config)

type DelayOption func(*config)

type OnRetryFn func(uint, error)

type RetryIfFn func(uint, error) bool

type DelayFn func(uint, error, *config) time.Duration

type JitterFn func(uint, error) time.Duration

var (
	defaultAttempts  = uint(10)
	defaultOnRetryFn = func(n uint, err error) {}
	defaultRetryIfFn = func(n uint, err error) bool {
		return !IsReconverableError(err)
	}
	defaultDelayFn = func(n uint, err error, c *config) time.Duration {
		return time.Duration(0)
	}
	defaultJitterTime = time.Duration(100 * time.Millisecond)
)

func SetMaxDelayTimeFn(maxDelayTime time.Duration) DelayOption {
	return func(c *config) {
		c.maxDelayTime = maxDelayTime
	}
}

func SetFixTimeFn(fixDelayTime time.Duration) DelayOption {
	return func(c *config) {
		c.delayTime = fixDelayTime
	}
}

func FixDelayFn(n uint, err error, c *config) time.Duration {
	return c.delayTime
}

func SetRamdomTimeFn(randomTime time.Duration) DelayOption {
	return func(c *config) {
		c.randomTime = randomTime
	}
}

func RandomDelayFn(n uint, err error, c *config) time.Duration {
	return time.Duration(rand.Int63n(int64(c.randomTime)))
}

func SetBackOffBeginTimeFn(backOffBeginTime time.Duration) DelayOption {
	return func(c *config) {
		c.delayTime = backOffBeginTime
	}
}

func BackOffDelayFn(n uint, err error, c *config) time.Duration {
	// 1 << 63 overflow signed int64
	max := uint(62)
	if c.delayTime == 0 {
		c.delayTime = 1
	}

	if c.maxBackOffN == 0 {
		c.maxBackOffN = max - uint(math.Floor(math.Log2(float64(c.delayTime))))
	}

	if n > c.maxBackOffN {
		n = c.maxBackOffN
	}

	return c.delayTime << n
}

func CombineDelayFn(delayFns ...DelayFn) DelayFn {
	return func(n uint, e error, c *config) time.Duration {
		var duration time.Duration
		for _, df := range delayFns {
			duration += df(n, e, c)
		}
		return duration
	}
}

func WithDelayFn(df DelayFn, opts ...DelayOption) Option {
	return func(c *config) {
		for _, opt := range opts {
			opt(c)
		}
		c.delayFn = func(n uint, e error, c *config) time.Duration {
			delayTime := df(n, e, c)
			if delayTime > c.maxDelayTime {
				return c.maxDelayTime
			}
			return delayTime
		}

	}
}

func WithContext(ctx context.Context) Option {
	return func(c *config) {
		c.ctx = ctx
	}
}

func WithAttempts(attempts uint) Option {
	return func(c *config) {
		c.attempts = attempts
	}
}

func WithOnRetryFn(onretryFn OnRetryFn) Option {
	return func(c *config) {
		c.onRetryFn = onretryFn
	}
}

func WithRetryIfFn(retryIfFn RetryIfFn) Option {
	return func(c *config) {
		c.retryIfFn = retryIfFn
	}
}

func WithLastErrorOnly(lastErrorOnly bool) Option {
	return func(c *config) {
		c.lastErrorOnly = lastErrorOnly
	}
}
