package logging

import "testing"

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
