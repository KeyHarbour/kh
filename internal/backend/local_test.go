package backend

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLocalReaderListAndGet(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.tfstate")
	data := []byte(`{"version":4,"terraform_version":"1.5.0"}`)
	if err := os.WriteFile(fp, data, 0o600); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[^.]+`) // simplistic pattern
	r := NewLocalReader(dir, re)
	objs, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	b, obj, err := r.Get(context.Background(), fp)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(b) != string(data) {
		t.Fatalf("Get data mismatch")
	}
	if obj.Size != int64(len(data)) {
		t.Fatalf("Size=%d want %d", obj.Size, len(data))
	}
	if obj.Checksum == "" {
		t.Fatalf("Checksum should be set")
	}
}

func TestLocalWriterPut(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "out.tfstate")
	w := &LocalWriter{}
	obj, err := w.Put(context.Background(), fp, []byte("{}"), false)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if obj.Key == "" || obj.Size == 0 || obj.Checksum == "" {
		t.Fatalf("invalid object: %+v", obj)
	}
}
