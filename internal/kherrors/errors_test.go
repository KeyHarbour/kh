package kherrors_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"kh/internal/exitcodes"
	"kh/internal/kherrors"
)

// ── Exit-code mapping ──────────────────────────────────────────────────────

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		def      kherrors.ErrorDef
		wantCode int
	}{
		{kherrors.ErrMissingFlag, exitcodes.ValidationError},
		{kherrors.ErrInvalidValue, exitcodes.ValidationError},
		{kherrors.ErrInvalidWorkspaceName, exitcodes.ValidationError},
		{kherrors.ErrConflictingFlags, exitcodes.ValidationError},
		{kherrors.ErrResourceConflict, exitcodes.ValidationError},
		{kherrors.ErrMissingToken, exitcodes.AuthError},
		{kherrors.ErrTokenInvalid, exitcodes.AuthError},
		{kherrors.ErrForbidden, exitcodes.AuthError},
		{kherrors.ErrBackendIO, exitcodes.BackendIOError},
		{kherrors.ErrAPIError, exitcodes.BackendIOError},
		{kherrors.ErrNotFound, exitcodes.UnknownError},
		{kherrors.ErrStateLocked, exitcodes.LockError},
		{kherrors.ErrPartialFailure, exitcodes.Partial},
		{kherrors.ErrInternal, exitcodes.UnknownError},
		{kherrors.ErrConfigLoad, exitcodes.UnknownError},
	}
	for _, tc := range cases {
		err := tc.def.New("test message")
		if got := err.ExitCode(); got != tc.wantCode {
			t.Errorf("%s.ExitCode() = %d, want %d", tc.def.Code, got, tc.wantCode)
		}
		// KHError must satisfy exitcodes.ExitCoder so root Execute() works.
		var ec exitcodes.ExitCoder
		if !errors.As(err, &ec) {
			t.Errorf("%s does not satisfy exitcodes.ExitCoder", tc.def.Code)
		}
	}
}

// ── ErrorDef constructors ─────────────────────────────────────────────────

func TestErrorDefNew(t *testing.T) {
	e := kherrors.ErrMissingToken.New("no token configured")
	if e.Code != "KH-AUTH-001" {
		t.Errorf("Code = %q, want KH-AUTH-001", e.Code)
	}
	if e.Category != kherrors.CategoryAuth {
		t.Errorf("Category = %q, want auth", e.Category)
	}
	if e.Message != "no token configured" {
		t.Errorf("Message = %q, want %q", e.Message, "no token configured")
	}
	if e.Hint == "" {
		t.Error("Hint should not be empty for ErrMissingToken")
	}
	if e.Error() != "no token configured" {
		t.Errorf("Error() = %q, want %q", e.Error(), "no token configured")
	}
}

func TestErrorDefNewf(t *testing.T) {
	e := kherrors.ErrNotFound.Newf("project %q not found", "abc-123")
	want := `project "abc-123" not found`
	if e.Message != want {
		t.Errorf("Message = %q, want %q", e.Message, want)
	}
}

func TestErrorDefWrap(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	e := kherrors.ErrBackendIO.Wrap("failed to connect to backend", cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if e.Unwrap() != cause {
		t.Error("Unwrap() should return the original cause")
	}
}

func TestErrorDefWrapf(t *testing.T) {
	cause := errors.New("timeout")
	e := kherrors.ErrBackendIO.Wrapf(cause, "list failed after %d retries", 3)
	if e.Message != "list failed after 3 retries" {
		t.Errorf("Message = %q", e.Message)
	}
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

// ── Classify ──────────────────────────────────────────────────────────────

func TestClassifyPassthrough(t *testing.T) {
	original := kherrors.ErrMissingToken.New("no token")
	got := kherrors.Classify(original)
	if got != original {
		t.Error("Classify should return the same *KHError when one is already in the chain")
	}
}

func TestClassifyWrappedKHError(t *testing.T) {
	inner := kherrors.ErrForbidden.New("forbidden")
	wrapped := fmt.Errorf("operation failed: %w", inner)
	got := kherrors.Classify(wrapped)
	if got != inner {
		t.Error("Classify should unwrap to find the inner *KHError")
	}
}

func TestClassifyPlainError(t *testing.T) {
	plain := errors.New("something unexpected")
	got := kherrors.Classify(plain)
	if got.Code != "KH-INT-001" {
		t.Errorf("Code = %q, want KH-INT-001", got.Code)
	}
	if got.Category != kherrors.CategoryInternal {
		t.Errorf("Category = %q, want internal", got.Category)
	}
}

// ── JSON marshaling ───────────────────────────────────────────────────────

func TestKHErrorJSON(t *testing.T) {
	e := kherrors.ErrMissingToken.New("not authenticated")
	envelope := map[string]any{"error": e}
	b, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got struct {
		Error struct {
			Code     string `json:"code"`
			Category string `json:"category"`
			Message  string `json:"message"`
			Hint     string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Error.Code != "KH-AUTH-001" {
		t.Errorf("code = %q, want KH-AUTH-001", got.Error.Code)
	}
	if got.Error.Category != "auth" {
		t.Errorf("category = %q, want auth", got.Error.Category)
	}
	if got.Error.Message != "not authenticated" {
		t.Errorf("message = %q, want %q", got.Error.Message, "not authenticated")
	}
	if got.Error.Hint == "" {
		t.Error("hint should be present for ErrMissingToken")
	}
}

func TestKHErrorJSONOmitsDocsURLWhenEmpty(t *testing.T) {
	e := kherrors.ErrBackendIO.New("connection refused")
	b, _ := json.Marshal(e)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	if _, ok := raw["docs_url"]; ok {
		t.Error("docs_url should be omitted when empty")
	}
}

func TestKHErrorJSONDoesNotLeakCause(t *testing.T) {
	cause := errors.New("internal secret: token=abc123")
	e := kherrors.ErrInternal.Wrap("unexpected error", cause)
	b, _ := json.Marshal(e)
	// The raw cause must not appear in the JSON output.
	if string(b) == "" {
		t.Fatal("marshal produced empty output")
	}
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	if _, ok := raw["cause"]; ok {
		t.Error("cause field must not be marshaled to JSON")
	}
}

// ── Secret redaction ──────────────────────────────────────────────────────

func TestRedact(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "KH_TOKEN assignment",
			input: "env var KH_TOKEN=supersecret123 was rejected",
			want:  "env var KH_TOKEN=[REDACTED] was rejected",
		},
		{
			name:  "TF_API_TOKEN assignment",
			input: "TF_API_TOKEN=mytoken failed",
			want:  "TF_API_TOKEN=[REDACTED] failed",
		},
		{
			name:  "Bearer token inline",
			input: "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig",
			want:  "Authorization: Bearer [REDACTED]",
		},
		{
			name:  "URL with embedded password",
			input: "connecting to https://user:s3cr3t@app.keyharbour.ca failed",
			want:  "connecting to https://user:[REDACTED]@app.keyharbour.ca failed",
		},
		{
			name:  "plain text without secrets",
			input: "project not found",
			want:  "project not found",
		},
		{
			name:  "KH_ENCRYPTION_KEY",
			input: "KH_ENCRYPTION_KEY=deadbeefdeadbeef1234567890abcdef",
			want:  "KH_ENCRYPTION_KEY=[REDACTED]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kherrors.Redact(tc.input)
			if got != tc.want {
				t.Errorf("\n got:  %q\n want: %q", got, tc.want)
			}
		})
	}
}

func TestRedactAppliedInNew(t *testing.T) {
	msg := "token KH_TOKEN=topsecretvalue in request"
	e := kherrors.ErrInternal.New(msg)
	if e.Message == msg {
		t.Error("New() should redact secrets from the message")
	}
	if e.Message != "token KH_TOKEN=[REDACTED] in request" {
		t.Errorf("Message = %q", e.Message)
	}
}
