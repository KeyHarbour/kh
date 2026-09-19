package workerpool

import (
	"context"
	"fmt"
	"sync"
)

type TaskFunc[T any] func(item T) error

type Result struct {
	Err error
}

// Run runs fn over items with at most 'concurrency' workers.
func Run[T any](items []T, concurrency int, fn TaskFunc[T]) []Result {
	return RunContext(context.Background(), items, concurrency, fn)
}

// RunContext runs fn over items with at most 'concurrency' workers and
// stops dispatching new work when ctx is canceled.
func RunContext[T any](ctx context.Context, items []T, concurrency int, fn TaskFunc[T]) []Result {
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
				if ctx.Err() != nil {
					res[idx].Err = ctx.Err()
					continue
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							res[idx].Err = fmt.Errorf("worker panic: %v", r)
						}
					}()
					res[idx].Err = fn(items[idx])
				}()
			}
		}()
	}
	for i := range items {
		if ctx.Err() != nil {
			for j := i; j < len(items); j++ {
				res[j].Err = ctx.Err()
			}
			break
		}
		ch <- i
	}
	close(ch)
	wg.Wait()
	return res
}
