package cli

import (
	"fmt"
	"io"
)

// validateAndSanitizeWorkspaceName checks if a workspace name contains invalid characters
// and returns the sanitized version along with a warning message if sanitization occurred
func validateAndSanitizeWorkspaceName(name string, stderr io.Writer) string {
	sanitized := sanitizeWorkspaceName(name)
	if sanitized != name {
		fmt.Fprintf(stderr, "Warning: Workspace name %q contains invalid characters (only alphanumeric allowed). Using sanitized name: %q\n", name, sanitized)
	}
	return sanitized
}
