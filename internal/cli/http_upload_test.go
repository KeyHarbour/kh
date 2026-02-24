package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHTTPUploadState_VerifyHeaderSent(t *testing.T) {
	// Create temp file with known contents
	content := []byte("test-state-content")
	tmp, err := os.CreateTemp(t.TempDir(), "state-*.tfstate")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}

	// Start a test server that validates the checksum header on PUT and
	// returns the last-written body on GET so the client can read it back.
	var seenHeader string
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			sum := sha256.Sum256(b)
			hexSum := hex.EncodeToString(sum[:])
			seenHeader = r.Header.Get("X-Checksum-Sha256")
			if seenHeader != hexSum {
				http.Error(w, "checksum mismatch", http.StatusBadRequest)
				return
			}
			lastBody = append([]byte(nil), b...)
			w.Header().Set("X-Checksum-Sha256", hexSum)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if lastBody == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.terraform.state+json;version=4")
			_, _ = w.Write(lastBody)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	// Build and run the command
	cmd := newHTTPUploadStateCmd()
	if err := cmd.Flags().Set("file", path); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("url", srv.URL+"/states/app/dev.tfstate"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("verify-after-upload", "true"); err != nil {
		t.Fatal(err)
	}

	// Ensure command has a context so RunE's use of cmd.Context() works
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// compute expected checksum
	s := sha256.Sum256(content)
	expected := hex.EncodeToString(s[:])
	if seenHeader != expected {
		t.Fatalf("expected header %s got %s", expected, seenHeader)
	}
}
