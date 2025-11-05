package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

type HTTPReader struct {
	URL string
}

func NewHTTPReader(url string) *HTTPReader { return &HTTPReader{URL: url} }

func (r *HTTPReader) List(ctx context.Context) ([]Object, error) {
	b, obj, err := r.Get(ctx, r.URL)
	if err != nil {
		return nil, err
	}
	_ = b
	return []Object{obj}, nil
}

func (r *HTTPReader) Get(ctx context.Context, key string) ([]byte, Object, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, Object{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, Object{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, Object{}, fmt.Errorf("GET %s: %s", r.URL, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Object{}, err
	}
	h := sha256.Sum256(data)
	obj := Object{Key: r.URL, Size: int64(len(data)), Checksum: hex.EncodeToString(h[:]), Workspace: "default", URL: r.URL}
	return data, obj, nil
}

type HTTPWriter struct {
	URL     string
	Headers map[string]string
}

func NewHTTPWriter(url string) *HTTPWriter { return &HTTPWriter{URL: url} }
func NewHTTPWriterWithHeaders(url string, headers map[string]string) *HTTPWriter {
	return &HTTPWriter{URL: url, Headers: headers}
}

func (w *HTTPWriter) Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error) {
	// Prefer explicit key (resolved URL) when provided, otherwise fall back to the writer's URL
	target := w.URL
	if key != "" {
		target = key
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, bytes.NewReader(data))
	if err != nil {
		return Object{}, err
	}
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Object{}, fmt.Errorf("PUT %s: %s", target, resp.Status)
	}
	// Prefer server-echoed checksum when present (server validated the upload).
	serverSum := resp.Header.Get("X-Checksum-Sha256")
	h := sha256.Sum256(data)
	localSum := hex.EncodeToString(h[:])
	checksum := localSum
	if serverSum != "" {
		checksum = serverSum
	}
	// Return object summary with the target URL and chosen checksum (server-echoed if available).
	obj := Object{Key: target, Size: int64(len(data)), Checksum: checksum, URL: target}
	return obj, nil
}
