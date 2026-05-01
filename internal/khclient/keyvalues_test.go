package khclient

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListKeyValues(t *testing.T) {
	var called bool
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/workspaces/ws/keyvalues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"key":"FOO","value":"bar","expires_at":null,"private":false},{"key":"SECRET","value":"s3cr3t","expires_at":null,"private":true}]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListKeyValues(context.Background(), "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("server not called")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Key != "FOO" || items[0].Value != "bar" || items[0].Private {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
	if items[1].Key != "SECRET" || !items[1].Private {
		t.Fatalf("unexpected item[1]: %+v", items[1])
	}
}

func TestGetKeyValue(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "hello")
	})

	c := New(config.Config{Endpoint: srv.URL})
	kv, err := c.GetKeyValue(context.Background(), "MY_KEY")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if kv.Key != "MY_KEY" {
		t.Fatalf("expected key to be set to MY_KEY, got %q", kv.Key)
	}
	if kv.Value != "hello" {
		t.Fatalf("expected value hello, got %q", kv.Value)
	}
	if string(kv.RawValue) != "hello" {
		t.Fatalf("expected raw value hello, got %q", string(kv.RawValue))
	}
}

func TestGetKeyValue_JSONResponse(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"value":"hello","expires_at":null,"private":false}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	kv, err := c.GetKeyValue(context.Background(), "MY_KEY")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if kv.Value != "hello" {
		t.Fatalf("expected value hello, got %q", kv.Value)
	}
	if string(kv.RawValue) != "hello" {
		t.Fatalf("expected raw value hello, got %q", string(kv.RawValue))
	}
}

func TestGetKeyValue_RequiresKey(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.GetKeyValue(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCreateKeyValue(t *testing.T) {
	var gotFields map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/ws/keyvalues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotFields = parseMultipartFields(t, r)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"accepted"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.CreateKeyValue(context.Background(), "ws", CreateKeyValueRequest{
		Key:     "NEW_KEY",
		Payload: "new-value",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotFields["key"] != "NEW_KEY" {
		t.Fatalf("expected key NEW_KEY in form, got: %#v", gotFields)
	}
	if gotFields["value"] != "new-value" {
		t.Fatalf("expected value new-value in form, got: %#v", gotFields)
	}
	if _, ok := gotFields["valuefile"]; ok {
		t.Fatalf("did not expect valuefile form field for regular values, got: %#v", gotFields)
	}
}

func TestCreateKeyValue_FromValueFileUsesValueFileField(t *testing.T) {
	var gotFields map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/ws/keyvalues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotFields = parseMultipartFields(t, r)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"accepted"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.CreateKeyValue(context.Background(), "ws", CreateKeyValueRequest{
		Key:             "NEW_KEY",
		Payload:         "new-value",
		PayloadFromFile: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotFields["value_file"] != "new-value" {
		t.Fatalf("expected value_file new-value in form, got: %#v", gotFields)
	}
	if _, ok := gotFields["value"]; ok {
		t.Fatalf("did not expect value form field for file-based values, got: %#v", gotFields)
	}
}

func TestUpdateKeyValue(t *testing.T) {
	var gotFields map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotFields = parseMultipartFields(t, r)
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"updated"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateKeyValue(context.Background(), "MY_KEY", UpdateKeyValueRequest{Payload: "updated"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotFields["key"] != "MY_KEY" {
		t.Fatalf("expected key MY_KEY in form, got: %#v", gotFields)
	}
	if gotFields["value"] != "updated" {
		t.Fatalf("expected value=updated in form, got: %#v", gotFields)
	}
	if _, ok := gotFields["value_file"]; ok {
		t.Fatalf("did not expect value_file form field for regular values, got: %#v", gotFields)
	}
}

func TestUpdateKeyValue_FromValueFileUsesValueFileField(t *testing.T) {
	var gotFields map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotFields = parseMultipartFields(t, r)
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"updated"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateKeyValue(context.Background(), "MY_KEY", UpdateKeyValueRequest{Payload: "updated", PayloadFromFile: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotFields["value_file"] != "updated" {
		t.Fatalf("expected value_file=updated in form, got: %#v", gotFields)
	}
	if _, ok := gotFields["value"]; ok {
		t.Fatalf("did not expect value form field for file-based values, got: %#v", gotFields)
	}
}

// TestUpdateKeyValue_SwitchFromFileToPlainValue verifies that a key originally
// created with a file payload can be updated with a plain text value. The
// backend accepts either payload form; the client must send the correct field
// (value, not value_file) and must NOT send both at the same time.
func TestUpdateKeyValue_SwitchFromFileToPlainValue(t *testing.T) {
	var calls []map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			calls = append(calls, parseMultipartFields(t, r))
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"status":"created"}`)
		case http.MethodPatch:
			calls = append(calls, parseMultipartFields(t, r))
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"status":"updated"}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	c := New(config.Config{Endpoint: srv.URL})

	// First call: create the key with a file payload.
	if err := c.CreateKeyValue(context.Background(), "ws", CreateKeyValueRequest{
		Key:             "CERT",
		Payload:         "-----BEGIN CERTIFICATE-----",
		PayloadFromFile: true,
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Second call: update the same key with a plain text value.
	if err := c.UpdateKeyValue(context.Background(), "CERT", UpdateKeyValueRequest{
		Payload:         "plain-text-replacement",
		PayloadFromFile: false,
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(calls))
	}

	// Create request must have used value_file, not value.
	createFields := calls[0]
	if createFields["value_file"] != "-----BEGIN CERTIFICATE-----" {
		t.Errorf("create: expected value_file field, got: %#v", createFields)
	}
	if _, ok := createFields["value"]; ok {
		t.Errorf("create: must not send both value and value_file, got: %#v", createFields)
	}

	// Update request must have used value, not value_file.
	updateFields := calls[1]
	if updateFields["value"] != "plain-text-replacement" {
		t.Errorf("update: expected value field, got: %#v", updateFields)
	}
	if _, ok := updateFields["value_file"]; ok {
		t.Errorf("update: must not send both value and value_file, got: %#v", updateFields)
	}
}

// TestUpdateKeyValue_SwitchFromPlainValueToFile verifies the reverse: a key
// originally stored as plain text can be updated with a file payload.
func TestUpdateKeyValue_SwitchFromPlainValueToFile(t *testing.T) {
	var calls []map[string]string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			calls = append(calls, parseMultipartFields(t, r))
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"status":"created"}`)
		case http.MethodPatch:
			calls = append(calls, parseMultipartFields(t, r))
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"status":"updated"}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	c := New(config.Config{Endpoint: srv.URL})

	// First call: create the key with a plain text value.
	if err := c.CreateKeyValue(context.Background(), "ws", CreateKeyValueRequest{
		Key:             "TOKEN",
		Payload:         "initial-token",
		PayloadFromFile: false,
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Second call: update with a file payload.
	if err := c.UpdateKeyValue(context.Background(), "TOKEN", UpdateKeyValueRequest{
		Payload:         "-----BEGIN PRIVATE KEY-----",
		PayloadFromFile: true,
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(calls))
	}

	createFields := calls[0]
	if createFields["value"] != "initial-token" {
		t.Errorf("create: expected value field, got: %#v", createFields)
	}
	if _, ok := createFields["value_file"]; ok {
		t.Errorf("create: must not send both value and value_file, got: %#v", createFields)
	}

	updateFields := calls[1]
	if updateFields["value_file"] != "-----BEGIN PRIVATE KEY-----" {
		t.Errorf("update: expected value_file field, got: %#v", updateFields)
	}
	if _, ok := updateFields["value"]; ok {
		t.Errorf("update: must not send both value and value_file, got: %#v", updateFields)
	}
}

func parseMultipartFields(t *testing.T, r *http.Request) map[string]string {
	t.Helper()
	ct := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("expected multipart/form-data, got %q", mediaType)
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		b, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read multipart value: %v", err)
		}
		fields[part.FormName()] = string(b)
	}
	return fields
}

func TestDeleteKeyValue(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteKeyValue(context.Background(), "MY_KEY"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteKeyValue_RequiresKey(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteKeyValue(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}
