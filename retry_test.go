package retry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDoDefaultTimes(t *testing.T) {
	retryNum := uint(0)
	err := Do(func() error {
		return errors.New("error")
	}, WithOnRetryFn(func(i uint, e error) {
		retryNum = i
	}))

	expectedErr := `Retry Error: 
# 0: error
# 1: error
# 2: error
# 3: error
# 4: error
# 5: error
# 6: error
# 7: error
# 8: error
# 9: error`
	assert.Equal(t, defaultAttempts-1, retryNum, fmt.Sprintf("shoud retry default %d times", defaultAttempts-1))
	assert.EqualError(t, err, expectedErr)
}

func TestDoRetryIf(t *testing.T) {
	retryNum := uint(0)
	retryTill := uint(5)
	_ = Do(func() error {
		return errors.New("error")
	}, WithOnRetryFn(func(i uint, e error) {
		retryNum = i
	}), WithRetryIfFn(func(i uint, e error) bool {
		return i <= retryTill
	}))

	assert.Equal(t, retryTill, retryNum, fmt.Sprintf("retry till %v times", retryTill))
}

func TestUnRecoverableError(t *testing.T) {
	retryNum := uint(0)
	expectErr := errors.New("error")
	err := Do(func() error {
		return UnrecoverableError(expectErr)
	}, WithOnRetryFn(func(i uint, e error) {
		retryNum = i
	}), WithLastErrorOnly(true))

	assert.Equal(t, uint(0), retryNum, fmt.Sprintf("unrecoverable, shouldn't retry"))
	assert.Equal(t, expectErr, err)
}

func TestContextCanceled(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		retryNum := uint(0)
		err := Do(func() error {
			return UnrecoverableError(errors.New("error"))
		}, WithOnRetryFn(func(i uint, e error) {
			retryNum = i
		}), WithContext(cancelCtx))

		assert.Equal(t, uint(0), retryNum, fmt.Sprintf("canceled, shouldn't retry"))
		assert.EqualError(t, err, "context canceled")
	})

	t.Run("context timeout", func(t *testing.T) {
		timedCtx, _ := context.WithTimeout(context.Background(), time.Second)

		retryNum := uint(0)
		err := Do(func() error {
			return errors.New("error")
		}, WithOnRetryFn(func(i uint, e error) {
			retryNum = i
		}), WithDelayFn(FixDelayFn, SetFixTimeFn(500*time.Millisecond)),
			WithContext(timedCtx),
			WithLastErrorOnly(true))

		assert.True(t, retryNum > 0, fmt.Sprintf("shouldn retry more than once when context timeout"))
		assert.EqualError(t, err, "context deadline exceeded")
	})
}

func TestDoFixDelayFn(t *testing.T) {
	start := time.Now()
	attempts := uint(2)
	expectRetryNum := uint(1)
	delayTime := time.Duration(100 * time.Millisecond)
	retryNum := uint(0)
	expectErr := errors.New("error")

	t.Run("fix delay", func(t *testing.T) {
		err := Do(func() error {
			return expectErr
		}, WithDelayFn(FixDelayFn, SetFixTimeFn(delayTime)),
			WithOnRetryFn(func(u uint, e error) {
				retryNum = u
			}),
			WithLastErrorOnly(true),
			WithAttempts(attempts))

		assert.Equal(t, expectRetryNum, retryNum, fmt.Sprintf("should retry %v time", attempts))
		// assert.LessOrEqual(t, time.Duration(int64(attempts)*int64(delayTime)), time.Since(start))
		assert.True(t, time.Now().After(start.Add(time.Duration(int64(attempts-1)*int64(delayTime)))), fmt.Sprintf("shoud run more than %v ms", int64(attempts-1)*int64(delayTime/time.Millisecond)))
		assert.Equal(t, expectErr, err)
	})
}

func TestDoRamdomDelayFn(t *testing.T) {
	start := time.Now()
	delayTime := time.Duration(100 * time.Millisecond)
	var retryNum uint
	attempts := uint(10)
	expectRetryNum := attempts - 1
	expectErr := errors.New("error")
	t.Run("random delay", func(t *testing.T) {
		err := Do(func() error {
			return expectErr
		}, WithDelayFn(RandomDelayFn, SetRamdomTimeFn(delayTime)),
			WithOnRetryFn(func(u uint, e error) {
				retryNum = u
			}),
			WithAttempts(attempts),
			WithLastErrorOnly(true))

		assert.Equal(t, expectRetryNum, retryNum, fmt.Sprintf("should retry %v time(s)", expectRetryNum))
		assert.True(t, time.Now().Before(start.Add(time.Duration(int64(expectRetryNum)*int64(delayTime)))), fmt.Sprintf("shoud run more than %v ms", int64(expectRetryNum)*int64(delayTime/time.Millisecond)))
		assert.EqualError(t, err, "error")
	})
}

func TestDoCombineDelayFn(t *testing.T) {
	start := time.Now()
	delayTime := time.Duration(100 * time.Millisecond)
	var retryNum uint
	attempts := uint(2)
	expectRetryNum := attempts - 1
	t.Run("combine delay", func(t *testing.T) {
		err := Do(func() error {
			return errors.New("error")
		}, WithDelayFn(CombineDelayFn(FixDelayFn, RandomDelayFn), SetFixTimeFn(delayTime), SetRamdomTimeFn(delayTime)),
			WithOnRetryFn(func(u uint, e error) {
				retryNum = u
			}),
			WithAttempts(attempts),
			WithLastErrorOnly(true))

		assert.Equal(t, expectRetryNum, retryNum, fmt.Sprintf("should retry %v time(s)", expectRetryNum))
		assert.True(t, time.Now().Before(start.Add(time.Duration(expectRetryNum)*2*delayTime)), fmt.Sprintf("shoud run more than %v ms", int64(time.Duration(expectRetryNum)*2*delayTime/time.Millisecond)))
		assert.True(t, time.Now().After(start.Add(time.Duration(expectRetryNum)*delayTime)), fmt.Sprintf("shoud run more than %v ms", int64(time.Duration(expectRetryNum)*delayTime/time.Millisecond)))
		assert.EqualError(t, err, "error")
	})
}

func TestErrorIs(t *testing.T) {
	var e Error
	expectErr := errors.New("error")
	closedErr := os.ErrClosed
	e = append(e, expectErr)
	e = append(e, closedErr)

	assert.True(t, errors.Is(e, expectErr))
	assert.True(t, errors.Is(e, closedErr))
	assert.False(t, errors.Is(e, errors.New("error")))
}

type fooErr struct{ str string }

func (e fooErr) Error() string {
	return e.str
}

type barErr struct{ str string }

func (e barErr) Error() string {
	return e.str
}

func TestErrorAs(t *testing.T) {
	var e Error
	fe := fooErr{str: "foo"}
	e = append(e, fe)

	var tf fooErr
	var tb barErr

	assert.True(t, errors.As(e, &tf))
	assert.False(t, errors.As(e, &tb))
	assert.Equal(t, "foo", tf.str)
}
