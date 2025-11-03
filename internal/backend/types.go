package backend

import (
	"context"
)

type Object struct {
	Key       string
	Size      int64
	Checksum  string // sha256 hex
	Workspace string
	Module    string
	URL       string
}

type Reader interface {
	List(ctx context.Context) ([]Object, error)
	Get(ctx context.Context, key string) ([]byte, Object, error)
}

type Writer interface {
	Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error)
}
