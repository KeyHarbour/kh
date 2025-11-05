package backend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTFCWriter_Put_Success(t *testing.T) {
	// Fake TFC API
	wsID := "ws-123"
	var captured struct {
		Auth string
		Body map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/organizations/"):
			w.Header().Set("Content-Type", "application/vnd.api+json")
			_, _ = w.Write([]byte(`{"data":{"id":"` + wsID + `"}}`))
			return
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v2/workspaces/") && strings.HasSuffix(r.URL.Path, "/state-versions"):
			captured.Auth = r.Header.Get("Authorization")
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			captured.Body = body
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	w := NewTFCWriter(srv.URL, "KeyHarbour", "ws-name", "tok")
	data := []byte(`{"serial":1,"lineage":"abc","terraform_version":"1.6.0"}`)
	obj, err := w.Put(context.Background(), "ignored", data, true)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if obj.Size == 0 || obj.Checksum == "" {
		t.Fatalf("invalid object: %+v", obj)
	}
	if captured.Auth == "" || !strings.HasPrefix(captured.Auth, "Bearer ") {
		t.Fatalf("missing auth header: %q", captured.Auth)
	}
	// Validate payload fields exist
	attrs := captured.Body["data"].(map[string]any)["attributes"].(map[string]any)
	if _, ok := attrs["md5"]; !ok {
		t.Fatalf("missing md5 in payload")
	}
	if s, ok := attrs["state"].(string); !ok || s == "" {
		t.Fatalf("missing state in payload")
	} else {
		// ensure state decodes
		if _, err := base64.StdEncoding.DecodeString(s); err != nil {
			t.Fatalf("state not base64: %v", err)
		}
	}
}
