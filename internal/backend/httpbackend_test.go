package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
