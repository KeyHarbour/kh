package cli

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"kh/internal/kherrors"
	"kh/internal/khclient"
)

// ── classifyError ─────────────────────────────────────────────────────────

func TestClassifyError_KHErrorPassthrough(t *testing.T) {
	original := kherrors.ErrMissingToken.New("no token")
	got := classifyError(original)
	if got != original {
		t.Error("classifyError should return a *KHError in the chain unchanged")
	}
}

func TestClassifyError_WrappedKHError(t *testing.T) {
	inner := kherrors.ErrForbidden.New("forbidden")
	wrapped := errors.New("outer: " + inner.Error())
	// Wrap with %w so errors.As can find it.
	got := classifyError(fmt.Errorf("context: %w", inner))
	if got.Code != "KH-PERM-001" {
		t.Errorf("Code = %q, want KH-PERM-001", got.Code)
	}
	_ = wrapped // suppress unused warning
}

func TestClassifyError_APIError401(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusUnauthorized, Message: "token expired"}
	got := classifyError(apiErr)
	if got.Code != "KH-AUTH-002" {
		t.Errorf("401 → Code = %q, want KH-AUTH-002", got.Code)
	}
	if got.Category != kherrors.CategoryAuth {
		t.Errorf("Category = %q, want auth", got.Category)
	}
}

func TestClassifyError_APIError403(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusForbidden, Message: "forbidden"}
	got := classifyError(apiErr)
	if got.Code != "KH-PERM-001" {
		t.Errorf("403 → Code = %q, want KH-PERM-001", got.Code)
	}
}

func TestClassifyError_APIError404(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusNotFound, Message: "not found"}
	got := classifyError(apiErr)
	if got.Code != "KH-NF-001" {
		t.Errorf("404 → Code = %q, want KH-NF-001", got.Code)
	}
}

func TestClassifyError_APIError409(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusConflict, Message: "locked"}
	got := classifyError(apiErr)
	if got.Code != "KH-CONF-001" {
		t.Errorf("409 → Code = %q, want KH-CONF-001", got.Code)
	}
}

func TestClassifyError_APIError500(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusInternalServerError, Message: "server error"}
	got := classifyError(apiErr)
	if got.Code != "KH-NET-002" {
		t.Errorf("500 → Code = %q, want KH-NET-002", got.Code)
	}
}

func TestClassifyError_PlainError(t *testing.T) {
	got := classifyError(errors.New("something went wrong"))
	if got.Code != "KH-INT-001" {
		t.Errorf("plain error → Code = %q, want KH-INT-001", got.Code)
	}
	if got.Category != kherrors.CategoryInternal {
		t.Errorf("Category = %q, want internal", got.Category)
	}
}

func TestClassifyError_RedactsSecrets(t *testing.T) {
	got := classifyError(errors.New("request failed KH_TOKEN=supersecret123"))
	if got.Message == "request failed KH_TOKEN=supersecret123" {
		t.Error("classifyError should redact secrets in plain error messages")
	}
}
