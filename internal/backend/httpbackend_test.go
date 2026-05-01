package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPReaderAndWriter(t *testing.T) {
	// Echo server that stores last body
	var last []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if last == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(last)
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			last = b
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	w := NewHTTPWriter(srv.URL)
	obj, err := w.Put(context.Background(), srv.URL, []byte("{}"), true)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if obj.Checksum == "" || obj.Size == 0 {
		t.Fatalf("writer returned invalid object: %+v", obj)
	}

	r := NewHTTPReader(srv.URL)
	data, ro, err := r.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("unexpected payload: %s", string(data))
	}
	if ro.Size == 0 || ro.Checksum == "" {
		t.Fatalf("reader object invalid: %+v", ro)
	}
}

func TestHTTPReaderListReturnsSingleObject(t *testing.T) {
	payload := []byte("terraform-state")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	r := NewHTTPReader(srv.URL)
	objects, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	if objects[0].Key != srv.URL {
		t.Fatalf("expected key %s, got %s", srv.URL, objects[0].Key)
	}
	if objects[0].Size != int64(len(payload)) {
		t.Fatalf("expected size %d, got %d", len(payload), objects[0].Size)
	}
}

func TestHTTPReaderListPropagatesGetError(t *testing.T) {
	r := &HTTPReader{URL: "://bad-url", HTTP: newHTTPClient()}

	objects, err := r.List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if objects != nil {
		t.Fatalf("expected nil objects on error, got %+v", objects)
	}
	if !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Fatalf("expected invalid URL error, got %v", err)
	}
}

func TestHTTPReaderGetReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	r := NewHTTPReader(srv.URL)
	_, _, err := r.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("expected status in error, got %v", err)
	}
}

func TestHTTPReaderGetReturnsReadError(t *testing.T) {
	r := &HTTPReader{
		URL: "http://example.test/state",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       errorReadCloser{err: errors.New("read failed")},
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, _, err := r.Get(context.Background(), r.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("expected read failure, got %v", err)
	}
}

func TestHTTPReaderGetReturnsRequestCreationError(t *testing.T) {
	r := &HTTPReader{URL: "://bad-url", HTTP: newHTTPClient()}

	_, _, err := r.Get(context.Background(), r.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Fatalf("expected invalid URL error, got %v", err)
	}
}

func TestHTTPReaderGetReturnsTransportError(t *testing.T) {
	r := &HTTPReader{
		URL: "http://example.test/state",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("transport failed")
		})},
	}

	_, _, err := r.Get(context.Background(), r.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "transport failed") {
		t.Fatalf("expected transport failure, got %v", err)
	}
}

func TestHTTPWriterPutUsesExplicitKeyAndHeaders(t *testing.T) {
	var gotPath string
	var gotHeader string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeader = r.Header.Get("X-Test-Header")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	w := NewHTTPWriterWithHeaders(srv.URL+"/default", map[string]string{"X-Test-Header": "present"})
	obj, err := w.Put(context.Background(), srv.URL+"/override", []byte("abc"), true)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if gotPath != "/override" {
		t.Fatalf("expected request path /override, got %s", gotPath)
	}
	if gotHeader != "present" {
		t.Fatalf("expected forwarded header, got %q", gotHeader)
	}
	if string(gotBody) != "abc" {
		t.Fatalf("expected body abc, got %q", string(gotBody))
	}
	if obj.Key != srv.URL+"/override" {
		t.Fatalf("expected key %s, got %s", srv.URL+"/override", obj.Key)
	}
	if obj.URL != srv.URL+"/override" {
		t.Fatalf("expected url %s, got %s", srv.URL+"/override", obj.URL)
	}
	if obj.Size != 3 {
		t.Fatalf("expected size 3, got %d", obj.Size)
	}
	if obj.Checksum == "" {
		t.Fatal("expected checksum to be set")
	}
}

func TestHTTPWriterPutReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	}))
	defer srv.Close()

	w := NewHTTPWriter(srv.URL)
	_, err := w.Put(context.Background(), "", []byte("abc"), true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "409 Conflict") {
		t.Fatalf("expected status in error, got %v", err)
	}
}

func TestHTTPWriterPutReturnsRequestCreationError(t *testing.T) {
	w := NewHTTPWriter("://bad-url")

	_, err := w.Put(context.Background(), "", []byte("abc"), true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing protocol scheme") {
		t.Fatalf("expected invalid URL error, got %v", err)
	}
}

func TestHTTPWriterPutReturnsTransportError(t *testing.T) {
	w := &HTTPWriter{
		URL: "http://example.test/state",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("transport failed")
		})},
	}

	_, err := w.Put(context.Background(), "", []byte("abc"), true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "transport failed") {
		t.Fatalf("expected transport failure, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type errorReadCloser struct {
	err error
}

func (e errorReadCloser) Read(_ []byte) (int, error) {
	return 0, e.err
}

func (e errorReadCloser) Close() error {
	return nil
}
