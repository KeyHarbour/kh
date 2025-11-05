package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPWriter_ServerEcho ensures that when the server validates and echoes
// the X-Checksum-Sha256 header the HTTPWriter.Put returns the server-echoed
// checksum (allowing the caller to skip a read-back verification).
func TestHTTPWriter_ServerEcho(t *testing.T) {
	var last []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read error", http.StatusInternalServerError)
				return
			}
			// compute sha256 of received payload
			h := sha256.Sum256(data)
			sum := hex.EncodeToString(h[:])

			// if client provided a checksum header, validate it
			if got := r.Header.Get("X-Checksum-Sha256"); got != "" && got != sum {
				http.Error(w, "checksum mismatch", http.StatusConflict)
				return
			}

			// persist and echo the checksum header as proof of validation
			last = data
			w.Header().Set("X-Checksum-Sha256", sum)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if last == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(last)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	// prepare a payload and its checksum
	payload := []byte(`{"example":"state"}`)
	h := sha256.Sum256(payload)
	checksum := hex.EncodeToString(h[:])

	headers := map[string]string{"X-Checksum-Sha256": checksum}
	w := NewHTTPWriterWithHeaders(srv.URL, headers)

	obj, err := w.Put(context.Background(), "", payload, false)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if obj.Checksum != checksum {
		t.Fatalf("expected returned checksum %s, got %s", checksum, obj.Checksum)
	}

	if obj.Key != srv.URL {
		t.Fatalf("expected key %s, got %s", srv.URL, obj.Key)
	}
}
