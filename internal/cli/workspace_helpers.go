package cli

import (
	"fmt"
	"io"

	"github.com/KeyHarbour/kh/internal/kherrors"
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
// to stderr if any characters were removed. A name that sanitizes to the empty
// string is rejected rather than passed on to workspace lookup or creation.
func validateAndSanitizeWorkspaceName(name string, stderr io.Writer) (string, error) {
	sanitized := sanitizeWorkspaceName(name)
	if sanitized == "" {
		return "", kherrors.ErrInvalidWorkspaceName.Newf("workspace name %q contains no alphanumeric characters", name)
	}
	if sanitized != name {
		fmt.Fprintf(stderr, "Warning: Workspace name %q contains invalid characters (only alphanumeric allowed). Using sanitized name: %q\n", name, sanitized)
	}
	return sanitized, nil
}

// checkWorkspaceNameCollisions reports names that are distinct at the source but
// collapse to the same workspace name once sanitized — "prod-1" and "prod_1"
// both become "prod1", which would silently target one workspace instead of two.
func checkWorkspaceNameCollisions(names []string) error {
	seen := make(map[string]string, len(names))
	for _, name := range names {
		sanitized := sanitizeWorkspaceName(name)
		if sanitized == "" {
			return kherrors.ErrInvalidWorkspaceName.Newf("workspace name %q contains no alphanumeric characters", name)
		}
		if first, exists := seen[sanitized]; exists && first != name {
			return kherrors.ErrInvalidWorkspaceName.Newf("workspace names %q and %q both sanitize to %q; rename one at the source to keep them distinct", first, name, sanitized)
		}
		seen[sanitized] = name
	}
	return nil
}
