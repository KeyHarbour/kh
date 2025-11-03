package logging

import "testing"

func TestSetDebugToggles(t *testing.T) {
	SetDebug(false)
	if Enabled() {
		t.Fatalf("expected disabled")
	}
	SetDebug(true)
	if !Enabled() {
		t.Fatalf("expected enabled")
	}
}
