package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type unrecoverableError struct {
	err error
}

func (e unrecoverableError) Error() string {
	return e.err.Error()
}

func UnrecoverableError(err error) unrecoverableError {
	return unrecoverableError{
		err: err,
	}
}

func IsReconverableError(err error) bool {
	ue := unrecoverableError{}
	return errors.As(err, &ue)
}

func UnwrapUnrecoverableError(err error) error {
	if IsReconverableError(err) {
		ue := err.(unrecoverableError)
		return ue.err
	}
	return err
}

// TODO cyx errors.Is & errors.As
type Error []error

func (e Error) Error() string {
	var res []string
	for i, v := range e {
		res = append(res, fmt.Sprintf("# %v: %v", i, v.Error()))
	}
	return fmt.Sprintf("Retry Error: \n%v", strings.Join(res, "\n"))
}

func (e Error) Is(target error) bool {
	for _, v := range e {
		if errors.Is(v, target) {
			return true
		}
	}
	return false
}

func (e Error) As(target interface{}) bool {
	for _, v := range e {
		if errors.As(v, target) {
			return true
		}
	}
	return false
}

type config struct {
	attempts      uint
	onRetryFn     OnRetryFn
	retryIfFn     RetryIfFn
	delayFn       DelayFn
	randomTime    time.Duration
	maxDelayTime  time.Duration
	maxBackOffN   uint
	delayTime     time.Duration
	lastErrorOnly bool
	ctx           context.Context
}

func Do(f func() error, opts ...Option) error {

	cfg := newDefaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	if err := cfg.ctx.Err(); err != nil {
		return err
	}

	if cfg.attempts == 0 {
		// infinite loop
		return nil
	}

	var errs Error
	if cfg.lastErrorOnly {
		errs = make(Error, 1)
	} else {
		errs = make(Error, cfg.attempts)
	}
	var n, lastErrIndex uint
	for ; n < cfg.attempts; n++ {
		err := f()

		if err == nil {
			return nil
		}

		if !cfg.lastErrorOnly {
			lastErrIndex = n
		}
		errs[lastErrIndex] = UnwrapUnrecoverableError(err)
		if !cfg.retryIfFn(n, err) {
			break
		}

		cfg.onRetryFn(n, err)

		if n == cfg.attempts-1 {
			break
		}

		select {
		case <-time.After(cfg.delayFn(n, err, cfg)):
			break
		case <-cfg.ctx.Done():
			errs[lastErrIndex] = UnwrapUnrecoverableError(cfg.ctx.Err())
			if cfg.lastErrorOnly {
				return errs[lastErrIndex]
			}
			return errs
		}
	}

	if cfg.lastErrorOnly {
		return errs[lastErrIndex]
	}
	return errs
}

func newDefaultConfig() *config {
	return &config{
		attempts:     defaultAttempts,
		onRetryFn:    defaultOnRetryFn,
		retryIfFn:    defaultRetryIfFn,
		delayFn:      defaultDelayFn,
		maxDelayTime: time.Duration(1<<63 - 1),
		ctx:          context.Background(),
	}
}
