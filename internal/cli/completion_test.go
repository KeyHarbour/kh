package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionArgsHelpfulErrors(t *testing.T) {
	root := &cobra.Command{Use: "kh"}
	c := newCompletionCmd(root)

	if err := c.Args(c, []string{}); err == nil ||
		(len(err.Error()) == 0 || !containsAll(err.Error(), []string{"exactly one", "bash", "zsh", "fish", "powershell"})) {
		t.Fatalf("expected helpful error for missing arg, got: %v", err)
	}

	if err := c.Args(c, []string{"ksh"}); err == nil ||
		(len(err.Error()) == 0 || !containsAll(err.Error(), []string{"invalid shell", "accepted values", "bash", "zsh", "fish", "powershell"})) {
		t.Fatalf("expected helpful error for invalid arg, got: %v", err)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool { return (len(sub) == 0) || (indexOf(s, sub) >= 0) })()
}

// tiny, allocation-free substring search for short test strings
func indexOf(s, sub string) int {
	// naive search is fine here
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
