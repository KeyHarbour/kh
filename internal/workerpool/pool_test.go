package workerpool

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunProcessesAllItems(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	var count int32
	results := Run(items, 2, func(i int) error {
		atomic.AddInt32(&count, 1)
		if i == 3 {
			return errors.New("boom")
		}
		return nil
	})
	if count != int32(len(items)) {
		t.Fatalf("processed %d items, want %d", count, len(items))
	}
	// item with value 3 is at index 2
	if results[2].Err == nil {
		t.Fatalf("expected error for item 3 at index 2")
	}
}

func TestRunRecoversPanicsPerItem(t *testing.T) {
	items := []int{1, 2, 3, 4}
	var nonPanicCount int32

	results := Run(items, 2, func(i int) error {
		if i == 3 {
			panic("boom")
		}
		atomic.AddInt32(&nonPanicCount, 1)
		return nil
	})

	if nonPanicCount != 3 {
		t.Fatalf("expected non-panicking items to keep processing, got %d", nonPanicCount)
	}

	if results[2].Err == nil {
		t.Fatal("expected panic item to be converted to an error")
	}
	if !strings.Contains(results[2].Err.Error(), "worker panic") {
		t.Fatalf("expected panic error marker, got %v", results[2].Err)
	}
}

func TestRunContextStopsDispatchOnCancel(t *testing.T) {
	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	ctx, cancel := context.WithCancel(context.Background())
	var processed int32

	results := RunContext(ctx, items, 1, func(i int) error {
		atomic.AddInt32(&processed, 1)
		if i == 2 {
			cancel()
		}
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	if processed >= int32(len(items)) {
		t.Fatalf("expected cancellation to stop dispatching, processed=%d", processed)
	}

	canceledCount := 0
	for _, r := range results {
		if errors.Is(r.Err, context.Canceled) {
			canceledCount++
		}
	}
	if canceledCount == 0 {
		t.Fatal("expected at least one context canceled result")
	}
}
