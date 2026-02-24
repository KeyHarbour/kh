package exitcodes

import (
	"errors"
	"testing"
)

func TestWithWrapsErrorAndCode(t *testing.T) {
	base := errors.New("boom")
	err := With(ValidationError, base)
	ce, ok := err.(ExitCoder)
	if !ok {
		t.Fatalf("expected ExitCoder, got %T", err)
	}
	if ce.ExitCode() != ValidationError {
		t.Fatalf("ExitCode=%d want %d", ce.ExitCode(), ValidationError)
	}
	if !errors.Is(err, base) {
		t.Fatalf("wrapped error does not contain base")
	}
}
