package workerpool

import (
	"sync"
)

type TaskFunc[T any] func(item T) error

type Result struct {
	Err error
}

// Run runs fn over items with at most 'concurrency' workers.
func Run[T any](items []T, concurrency int, fn TaskFunc[T]) []Result {
	if concurrency <= 0 {
		concurrency = 1
	}
	res := make([]Result, len(items))
	ch := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range ch {
				res[idx].Err = fn(items[idx])
			}
		}()
	}
	for i := range items {
		ch <- i
	}
	close(ch)
	wg.Wait()
	return res
}
