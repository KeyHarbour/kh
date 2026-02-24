package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

type LocalReader struct {
	Path             string // file or directory
	WorkspacePattern *regexp.Regexp
}

func NewLocalReader(path string, workspacePattern *regexp.Regexp) *LocalReader {
	return &LocalReader{Path: path, WorkspacePattern: workspacePattern}
}

func (r *LocalReader) List(ctx context.Context) ([]Object, error) {
	var objs []Object
	info, err := os.Stat(r.Path)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		obj, err := r.inspectFile(r.Path)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
		return objs, nil
	}
	// Walk directory, pick *.tfstate
	err = filepath.WalkDir(r.Path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) == ".tfstate" {
			obj, ierr := r.inspectFile(p)
			if ierr != nil {
				return ierr
			}
			objs = append(objs, obj)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return objs, nil
}

func (r *LocalReader) inspectFile(p string) (Object, error) {
	f, err := os.Open(p)
	if err != nil {
		return Object{}, err
	}
	defer f.Close()
	h := sha256.New()
	sz, err := io.Copy(h, f)
	if err != nil {
		return Object{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	ws := "default"
	// naive workspace from parent dir or filename when matching pattern
	if r.WorkspacePattern != nil {
		base := filepath.Base(p)
		if m := r.WorkspacePattern.FindString(base); m != "" {
			ws = m
		}
	}
	return Object{Key: p, Size: sz, Checksum: sum, Workspace: ws, Module: "", URL: "file://" + p}, nil
}

func (r *LocalReader) Get(ctx context.Context, key string) ([]byte, Object, error) {
	b, err := os.ReadFile(key)
	if err != nil {
		return nil, Object{}, err
	}
	obj, err := r.inspectFile(key)
	return b, obj, err
}

// LocalWriter writes to a file path (exact path, parent dirs created).

type LocalWriter struct{}

func (w *LocalWriter) Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error) {
	if !overwrite {
		if _, err := os.Stat(key); err == nil {
			return Object{}, os.ErrExist
		}
	}
	if err := os.MkdirAll(filepath.Dir(key), 0o755); err != nil {
		return Object{}, err
	}
	if err := os.WriteFile(key, data, 0o600); err != nil {
		return Object{}, err
	}
	lr := NewLocalReader(key, nil)
	_, obj, err := lr.Get(ctx, key)
	return obj, err
}
