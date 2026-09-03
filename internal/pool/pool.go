package pool

import (
	"sync"
)

func Run[T any](n int, next func() (T, bool), handle func(T)) {
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for {
				t, ok := next()
				if !ok {
					// If no more tasks are left, return so the goroutine can be killed
					return
				}
				handle(t)
			}
		})
	}
	wg.Wait()
}
