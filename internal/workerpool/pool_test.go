package workerpool

import (
	"errors"
	"sync/atomic"
	"testing"
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
