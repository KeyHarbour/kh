package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewTFCReader_DefaultsAndTrim(t *testing.T) {
	r := NewTFCReader("", "org", "ws", "tok")
	if r.Host != "https://app.terraform.io" {
		t.Fatalf("expected default host, got %s", r.Host)
	}
	if r.HTTP == nil {
		t.Fatal("expected HTTP client")
	}

	r = NewTFCReader("https://example.test///", "org", "ws", "tok")
	if r.Host != "https://example.test" {
		t.Fatalf("expected trimmed host, got %s", r.Host)
	}
}

func TestTFCReaderList(t *testing.T) {
	t.Run("requires org and workspace", func(t *testing.T) {
		r := NewTFCReader("https://example.test", "", "", "tok")
		objects, err := r.List(context.Background())
		if err == nil || !strings.Contains(err.Error(), "org and workspace are required") {
			t.Fatalf("expected validation error, got %v", err)
		}
		if objects != nil {
			t.Fatalf("expected nil objects, got %+v", objects)
		}
	})

	t.Run("returns workspace object", func(t *testing.T) {
		r := NewTFCReader("https://example.test/", "org", "ws", "tok")
		objects, err := r.List(context.Background())
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if len(objects) != 1 {
			t.Fatalf("expected 1 object, got %d", len(objects))
		}
		if objects[0].Key != "ws" || objects[0].Workspace != "ws" {
			t.Fatalf("unexpected object %+v", objects[0])
		}
		if objects[0].URL != "https://example.test/org/workspaces/ws" {
			t.Fatalf("unexpected URL %s", objects[0].URL)
		}
	})
}

func TestTFCReaderGet_Success(t *testing.T) {
	var authHeaders []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/v2/organizations/org/workspaces/ws":
			tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}})
		case "/api/v2/workspaces/ws-123/current-state-version":
			tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"attributes": map[string]any{"hosted-state-download-url": srv.URL + "/download/state"},
				},
			})
		case "/download/state":
			if got := r.Header.Get("Accept"); got != "application/octet-stream" {
				t.Fatalf("expected octet-stream accept header, got %q", got)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("terraform-state"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := NewTFCReader(srv.URL, "org", "ws", "tok")
	data, obj, err := r.Get(context.Background(), "state-key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(data) != "terraform-state" {
		t.Fatalf("unexpected data %q", string(data))
	}
	if obj.Key != "state-key" || obj.Workspace != "ws" {
		t.Fatalf("unexpected object %+v", obj)
	}
	if obj.URL != srv.URL+"/download/state" {
		t.Fatalf("unexpected URL %s", obj.URL)
	}
	if obj.Size != int64(len(data)) {
		t.Fatalf("unexpected size %d", obj.Size)
	}
	if len(authHeaders) != 3 {
		t.Fatalf("expected 3 authenticated requests, got %d", len(authHeaders))
	}
	for _, header := range authHeaders {
		if header != "Bearer tok" {
			t.Fatalf("expected bearer token header, got %q", header)
		}
	}
}

func TestTFCReaderGet_ErrorPaths(t *testing.T) {
	t.Run("workspace lookup fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", http.StatusNotFound)
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "tfc workspace get") {
			t.Fatalf("expected workspace lookup error, got %v", err)
		}
	})

	t.Run("current state lookup fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/organizations/org/workspaces/ws":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}})
			case "/api/v2/workspaces/ws-123/current-state-version":
				http.Error(w, "boom", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "tfc current-state-version") {
			t.Fatalf("expected current state error, got %v", err)
		}
	})

	t.Run("download request creation fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/organizations/org/workspaces/ws":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}})
			case "/api/v2/workspaces/ws-123/current-state-version":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
					"data": map[string]any{
						"attributes": map[string]any{"hosted-state-download-url": "://bad-download"},
					},
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "missing protocol scheme") {
			t.Fatalf("expected download request error, got %v", err)
		}
	})

	t.Run("download status error", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/organizations/org/workspaces/ws":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}})
			case "/api/v2/workspaces/ws-123/current-state-version":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
					"data": map[string]any{
						"attributes": map[string]any{"hosted-state-download-url": srv.URL + "/download/state"},
					},
				})
			case "/download/state":
				http.Error(w, "denied", http.StatusUnauthorized)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "GET state: 401 Unauthorized") {
			t.Fatalf("expected download status error, got %v", err)
		}
	})

	t.Run("download transport error", func(t *testing.T) {
		r := &TFCReader{
			Host:      "https://example.test",
			Org:       "org",
			Workspace: "ws",
			Token:     "tok",
			HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/api/v2/organizations/org/workspaces/ws":
					return tfcJSONResponse(t, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}}), nil
				case "/api/v2/workspaces/ws-123/current-state-version":
					return tfcJSONResponse(t, http.StatusOK, map[string]any{
						"data": map[string]any{"attributes": map[string]any{"hosted-state-download-url": "https://example.test/download/state"}},
					}), nil
				case "/download/state":
					return nil, errors.New("download failed")
				default:
					return nil, errors.New("unexpected path")
				}
			})},
		}

		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "download failed") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})

	t.Run("download read error", func(t *testing.T) {
		r := &TFCReader{
			Host:      "https://example.test",
			Org:       "org",
			Workspace: "ws",
			Token:     "tok",
			HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/api/v2/organizations/org/workspaces/ws":
					return tfcJSONResponse(t, http.StatusOK, map[string]any{"data": map[string]any{"id": "ws-123"}}), nil
				case "/api/v2/workspaces/ws-123/current-state-version":
					return tfcJSONResponse(t, http.StatusOK, map[string]any{
						"data": map[string]any{"attributes": map[string]any{"hosted-state-download-url": "https://example.test/download/state"}},
					}), nil
				case "/download/state":
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     make(http.Header),
						Body:       errorReadCloser{err: errors.New("read failed")},
					}, nil
				default:
					return nil, errors.New("unexpected path")
				}
			})},
		}

		_, _, err := r.Get(context.Background(), "state-key")
		if err == nil || !strings.Contains(err.Error(), "read failed") {
			t.Fatalf("expected read error, got %v", err)
		}
	})
}

func TestTFCReaderGetWorkspaceID(t *testing.T) {
	t.Run("transport error", func(t *testing.T) {
		r := &TFCReader{
			Host:  "https://example.test",
			Token: "tok",
			HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("transport failed")
			})},
		}

		_, err := r.getWorkspaceID(context.Background(), "org", "ws")
		if err == nil || !strings.Contains(err.Error(), "transport failed") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":`))
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.getWorkspaceID(context.Background(), "org", "ws")
		if err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{"data": map[string]any{}})
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.getWorkspaceID(context.Background(), "org", "ws")
		if err == nil || !strings.Contains(err.Error(), "workspace id not found") {
			t.Fatalf("expected missing id error, got %v", err)
		}
	})
}

func TestTFCReaderGetCurrentStateDownloadURL(t *testing.T) {
	t.Run("transport error", func(t *testing.T) {
		r := &TFCReader{
			Host:  "https://example.test",
			Token: "tok",
			HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("transport failed")
			})},
		}

		_, err := r.getCurrentStateDownloadURL(context.Background(), "ws-123")
		if err == nil || !strings.Contains(err.Error(), "transport failed") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":`))
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.getCurrentStateDownloadURL(context.Background(), "ws-123")
		if err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("missing download url", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
				"data": map[string]any{"attributes": map[string]any{}},
			})
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.getCurrentStateDownloadURL(context.Background(), "ws-123")
		if err == nil || !strings.Contains(err.Error(), "no current state version") {
			t.Fatalf("expected missing URL error, got %v", err)
		}
	})
}

func TestTFCReaderListAllWorkspaces(t *testing.T) {
	t.Run("requires org", func(t *testing.T) {
		r := NewTFCReader("https://example.test", "", "ws", "tok")
		workspaces, err := r.ListAllWorkspaces(context.Background())
		if err == nil || !strings.Contains(err.Error(), "org is required") {
			t.Fatalf("expected validation error, got %v", err)
		}
		if workspaces != nil {
			t.Fatalf("expected nil workspaces, got %+v", workspaces)
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		var pages []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pages = append(pages, r.URL.RawQuery)
			switch r.URL.Query().Get("page[number]") {
			case "1":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
					"data": []map[string]any{{"id": "ws-1", "attributes": map[string]any{"name": "one"}}},
					"meta": map[string]any{"pagination": map[string]any{"current-page": 1, "total-pages": 2}},
				})
			case "2":
				tfcWriteJSONStatus(t, w, http.StatusOK, map[string]any{
					"data": []map[string]any{{"id": "ws-2", "attributes": map[string]any{"name": "two"}}},
					"meta": map[string]any{"pagination": map[string]any{"current-page": 2, "total-pages": 2}},
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		workspaces, err := r.ListAllWorkspaces(context.Background())
		if err != nil {
			t.Fatalf("ListAllWorkspaces error: %v", err)
		}
		if len(workspaces) != 2 {
			t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
		}
		if workspaces[0].ID != "ws-1" || workspaces[1].Name != "two" {
			t.Fatalf("unexpected workspaces %+v", workspaces)
		}
		if len(pages) != 2 {
			t.Fatalf("expected 2 page requests, got %d", len(pages))
		}
	})

	t.Run("bad status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.ListAllWorkspaces(context.Background())
		if err == nil || !strings.Contains(err.Error(), "tfc list workspaces") {
			t.Fatalf("expected list error, got %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":`))
		}))
		defer srv.Close()

		r := NewTFCReader(srv.URL, "org", "ws", "tok")
		_, err := r.ListAllWorkspaces(context.Background())
		if err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("request creation error", func(t *testing.T) {
		r := NewTFCReader("://bad-host", "org", "ws", "tok")
		_, err := r.ListAllWorkspaces(context.Background())
		if err == nil || !strings.Contains(err.Error(), "missing protocol scheme") {
			t.Fatalf("expected request creation error, got %v", err)
		}
	})

	t.Run("transport error", func(t *testing.T) {
		r := &TFCReader{
			Host:  "https://example.test",
			Org:   "org",
			Token: "tok",
			HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("transport failed")
			})},
		}

		_, err := r.ListAllWorkspaces(context.Background())
		if err == nil || !strings.Contains(err.Error(), "transport failed") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})
}

func tfcWriteJSONStatus(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}

func tfcJSONResponse(t *testing.T, status int, v any) *http.Response {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	resp := &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
	resp.Header.Set("Content-Type", "application/vnd.api+json")
	return resp
}
