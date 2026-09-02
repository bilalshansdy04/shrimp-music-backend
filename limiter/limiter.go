package limiter

import (
	"context"
	"errors"
)

type Semaphore chan struct{}

func New(max int) Semaphore {
	return make(Semaphore, max)
}

func (s Semaphore) Acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.New("timeout or canceled while waiting for resources")
	}
}

func (s Semaphore) Release() {
	<-s
}
