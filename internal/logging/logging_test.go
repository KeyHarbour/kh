package logging

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestSetDebugToggles(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })

	SetDebug(false)
	if Enabled() {
		t.Fatalf("expected disabled")
	}
	SetDebug(true)
	if !Enabled() {
		t.Fatalf("expected enabled")
	}
}

func TestDebugf_Disabled(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(false)

	out := captureStderr(t, func() {
		Debugf("should not appear: %s", "value")
	})
	if out != "" {
		t.Fatalf("expected no output when disabled, got: %q", out)
	}
}

func TestDebugf_Enabled(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(true)

	out := captureStderr(t, func() {
		Debugf("hello %s", "world")
	})

	if !strings.Contains(out, "[DEBUG]") {
		t.Fatalf("expected [DEBUG] prefix, got: %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected formatted message, got: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected newline at end, got: %q", out)
	}
}

func TestDebug_Disabled(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(false)

	out := captureStderr(t, func() {
		Debug("should not appear")
	})
	if out != "" {
		t.Fatalf("expected no output when disabled, got: %q", out)
	}
}

func TestDebug_Enabled(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(true)

	out := captureStderr(t, func() {
		Debug("foo", "bar")
	})

	if !strings.Contains(out, "[DEBUG]") {
		t.Fatalf("expected [DEBUG] prefix, got: %q", out)
	}
	if !strings.Contains(out, "foo bar") {
		t.Fatalf("expected space-joined args, got: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected newline at end, got: %q", out)
	}
}

func TestDebug_SingleArg(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(true)

	out := captureStderr(t, func() {
		Debug("only one")
	})

	if !strings.Contains(out, "only one") {
		t.Fatalf("expected message, got: %q", out)
	}
}
