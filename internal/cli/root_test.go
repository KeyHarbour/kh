package cli

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"kh/internal/khclient"
	"kh/internal/kherrors"
	"kh/internal/logging"
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

func TestClassifyError_APIError423(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: 423, Message: "locked"}
	got := classifyError(apiErr)
	if got.Code != "KH-CONF-001" {
		t.Errorf("423 → Code = %q, want KH-CONF-001", got.Code)
	}
}

func TestClassifyError_APIErrorUnhandledStatus(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusBadRequest, Message: "bad request"}
	got := classifyError(apiErr)
	if got.Code != "KH-VAL-002" {
		t.Errorf("400 → Code = %q, want KH-VAL-002", got.Code)
	}
}

func TestClassifyError_APIError422(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusUnprocessableEntity, Message: "validation failed"}
	got := classifyError(apiErr)
	if got.Code != "KH-VAL-002" {
		t.Errorf("422 → Code = %q, want KH-VAL-002", got.Code)
	}
}

func TestClassifyError_APIError429(t *testing.T) {
	apiErr := khclient.APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited"}
	got := classifyError(apiErr)
	if got.Code != "KH-NET-002" {
		t.Errorf("429 → Code = %q, want KH-NET-002", got.Code)
	}
}

// ── newRootCmd ────────────────────────────────────────────────────────────

func TestNewRootCmd_RegistersSubcommands(t *testing.T) {
	root := newRootCmd()
	seen := map[string]bool{}
	for _, sub := range root.Commands() {
		seen[sub.Name()] = true
	}
	for _, want := range []string{"auth", "tf", "project", "workspace", "kv", "config", "license", "completion"} {
		if !seen[want] {
			t.Errorf("expected subcommand %q to be registered", want)
		}
	}
}

func TestNewRootCmd_Run_PrintsHelp(t *testing.T) {
	root := newRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.Run(root, nil)
	if !strings.Contains(out.String(), "kh") {
		t.Errorf("expected help output, got %q", out.String())
	}
}

func TestNewRootCmd_PersistentPreRun_KHInsecureWarning(t *testing.T) {
	t.Setenv("KH_INSECURE", "1")
	root := newRootCmd()
	errBuf := &bytes.Buffer{}
	root.SetErr(errBuf)
	root.PersistentPreRun(root, nil)
	if !strings.Contains(errBuf.String(), "TLS certificate verification is disabled") {
		t.Errorf("expected insecure warning in stderr, got %q", errBuf.String())
	}
}

func TestNewRootCmd_PersistentPreRun_KHDebugEnabled(t *testing.T) {
	t.Cleanup(func() { logging.SetDebug(false) })
	t.Setenv("KH_DEBUG", "1")
	root := newRootCmd()
	root.SetErr(&bytes.Buffer{})
	root.PersistentPreRun(root, nil)
	if !logging.Enabled() {
		t.Error("expected debug logging to be enabled via KH_DEBUG=1")
	}
}

func TestNewRootCmd_PersistentPreRun_KHOutputFallback(t *testing.T) {
	orig := outputFormat
	t.Cleanup(func() { outputFormat = orig })
	t.Setenv("KH_OUTPUT", "json")
	root := newRootCmd()
	root.SetErr(&bytes.Buffer{})
	root.PersistentPreRun(root, nil)
	if outputFormat != "json" {
		t.Errorf("expected outputFormat=json from KH_OUTPUT env, got %q", outputFormat)
	}
}

// ── Execute ───────────────────────────────────────────────────────────────

func TestExecute_ReturnsZeroOnHelp(t *testing.T) {
	// Suppress help output written to os.Stdout by cobra.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
		w.Close()
		r.Close()
	})

	code := Execute()
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestExecute_ReturnsNonZeroOnError(t *testing.T) {
	origArgs := os.Args
	os.Args = []string{"kh", "--unknown-flag-xyz-test"}
	t.Cleanup(func() { os.Args = origArgs })

	// Suppress error output to stderr.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
		w.Close()
		r.Close()
	})

	code := Execute()
	if code == 0 {
		t.Error("expected non-zero exit code for unknown flag")
	}
}

func TestExecute_JSONErrorOutput(t *testing.T) {
	// Use "project show" with no project ref — always fails from RunE without
	// network, and doesn't shadow the root --output persistent flag.
	origArgs := os.Args
	origFormat := outputFormat
	os.Args = []string{"kh", "--output", "json", "project", "show"}
	t.Setenv("KH_PROJECT", "")
	t.Cleanup(func() {
		os.Args = origArgs
		outputFormat = origFormat
	})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	Execute()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	if !strings.Contains(buf.String(), `"error"`) {
		t.Errorf("expected JSON error envelope in stderr, got %q", buf.String())
	}
}

// ── effectiveOutputFormat / shadowed --output ─────────────────────────────

// executeCapturingStderr runs Execute() with the given argv and returns
// whatever it wrote to stderr.
func executeCapturingStderr(t *testing.T, argv ...string) string {
	t.Helper()

	origArgs := os.Args
	origFormat := outputFormat
	os.Args = argv
	t.Cleanup(func() {
		os.Args = origArgs
		outputFormat = origFormat
	})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	Execute()

	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	r.Close()
	return buf.String()
}

// Commands that declare their own local --output/-o flag shadow root's
// persistent flag, so the package-level outputFormat keeps its default and
// errors used to come out as plain text no matter how --output was spelled.
// "project ls" has a local -o and fails in RunE without touching the network.
func TestExecute_JSONErrorOutput_ShadowedLocalOutputFlag(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"persistent position", []string{"kh", "--output", "json", "project", "ls"}},
		{"local position", []string{"kh", "project", "ls", "--output", "json"}},
		{"short flag", []string{"kh", "project", "ls", "-o", "json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KH_ORG", "")
			out := executeCapturingStderr(t, tc.argv...)
			if !strings.Contains(out, `"error"`) || !strings.Contains(out, `"code"`) {
				t.Errorf("expected JSON error envelope in stderr, got %q", out)
			}
		})
	}
}

// The default stays human-readable: no --output means no envelope.
func TestExecute_TableErrorOutput_ShadowedLocalOutputFlag(t *testing.T) {
	t.Setenv("KH_ORG", "")
	out := executeCapturingStderr(t, "kh", "project", "ls")
	if strings.Contains(out, `"error"`) {
		t.Errorf("expected plain-text error, got JSON envelope: %q", out)
	}
	if !strings.Contains(out, "--org is required") {
		t.Errorf("expected the validation message in stderr, got %q", out)
	}
}

func TestEffectiveOutputFormat(t *testing.T) {
	origFormat := outputFormat
	t.Cleanup(func() { outputFormat = origFormat })

	t.Run("nil command falls back to outputFormat", func(t *testing.T) {
		outputFormat = "json"
		if got := effectiveOutputFormat(nil); got != "json" {
			t.Errorf("got %q, want json", got)
		}
	})

	t.Run("unset local flag falls back to outputFormat", func(t *testing.T) {
		root := newRootCmd()
		sub, _, err := root.Find([]string{"project", "ls"})
		if err != nil {
			t.Fatal(err)
		}
		// newRootCmd binds the persistent flag's default into outputFormat, so
		// set it afterwards to stand in for KH_OUTPUT or the persistent flag.
		outputFormat = "json"
		if got := effectiveOutputFormat(sub); got != "json" {
			t.Errorf("got %q, want json (inherited)", got)
		}
	})

	t.Run("set local flag wins over outputFormat", func(t *testing.T) {
		root := newRootCmd()
		sub, _, err := root.Find([]string{"project", "ls"})
		if err != nil {
			t.Fatal(err)
		}
		outputFormat = "table"
		if err := sub.Flags().Set("output", "json"); err != nil {
			t.Fatal(err)
		}
		if got := effectiveOutputFormat(sub); got != "json" {
			t.Errorf("got %q, want json (local flag)", got)
		}
	})
}
