package pool

import (
	"context"
	"sync"
)

func Run[T any](ctx context.Context, n int, tasks <-chan T, handle func(T)) {
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for {
				// When the context is cancelled stop pulling tasks from the channel
				// we don't want to process more tasks.
				// This will kill the goroutine so it's not leaked
				select {
				case t, ok := <-tasks:
					if !ok {
						// If no more tasks are left, return so the goroutine can be killed
						return
					}
					handle(t)
				case <-ctx.Done():
					return
				}
			}
		})
	}
	wg.Wait()
}
