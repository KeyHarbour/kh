package cli

import (
	"fmt"
	"io"
)

// sanitizeWorkspaceName strips any character that is not a letter or digit.
// KeyHarbour workspace names must be alphanumeric only.
func sanitizeWorkspaceName(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}

// validateAndSanitizeWorkspaceName sanitizes a workspace name and prints a warning
// to stderr if any characters were removed.
func validateAndSanitizeWorkspaceName(name string, stderr io.Writer) string {
	sanitized := sanitizeWorkspaceName(name)
	if sanitized != name {
		fmt.Fprintf(stderr, "Warning: Workspace name %q contains invalid characters (only alphanumeric allowed). Using sanitized name: %q\n", name, sanitized)
	}
	return sanitized
}
