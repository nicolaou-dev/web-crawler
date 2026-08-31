package pool

import "sync"

func Run[T any](n int, tasks <-chan T, handle func(T)) {
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for t := range tasks {
				handle(t)
			}
		})
	}
	wg.Wait()
}
