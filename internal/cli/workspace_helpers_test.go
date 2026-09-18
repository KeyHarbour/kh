package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"kh/internal/exitcodes"
	"kh/internal/kherrors"
)

func TestValidateAndSanitizeWorkspaceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"already clean", "prod1", "prod1", false},
		{"strips separators", "prod-1", "prod1", false},
		{"strips underscores", "prod_1", "prod1", false},
		{"no alphanumerics", "---", "", true},
		{"empty input", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndSanitizeWorkspaceName(tt.input, io.Discard)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAndSanitizeWorkspaceName_WarnsOnce(t *testing.T) {
	var sb strings.Builder
	if _, err := validateAndSanitizeWorkspaceName("prod-1", &sb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sb.String(), `Using sanitized name: "prod1"`) {
		t.Errorf("expected sanitization warning, got %q", sb.String())
	}

	sb.Reset()
	if _, err := validateAndSanitizeWorkspaceName("prod1", &sb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sb.String() != "" {
		t.Errorf("expected no warning for a clean name, got %q", sb.String())
	}
}

func TestCheckWorkspaceNameCollisions(t *testing.T) {
	tests := []struct {
		name    string
		names   []string
		wantErr bool
	}{
		{"distinct after sanitizing", []string{"prod", "staging"}, false},
		{"same name repeated", []string{"prod", "prod"}, false},
		{"empty set", nil, false},
		{"separator collision", []string{"prod-1", "prod_1"}, true},
		{"three-way collision", []string{"prod1", "prod-1", "prod.1"}, true},
		{"sanitizes to empty", []string{"prod", "---"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkWorkspaceNameCollisions(tt.names)
			if tt.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got %v", tt.wantErr, err)
			}
		})
	}
}

// Collisions and empty names are validation errors, so they must carry the
// validation exit code rather than a generic failure.
func TestWorkspaceNameErrorsAreValidation(t *testing.T) {
	errs := []error{
		func() error { _, err := validateAndSanitizeWorkspaceName("---", io.Discard); return err }(),
		checkWorkspaceNameCollisions([]string{"prod-1", "prod_1"}),
	}
	for _, err := range errs {
		var khErr *kherrors.KHError
		if !errors.As(err, &khErr) {
			t.Fatalf("expected *kherrors.KHError, got %T", err)
		}
		if khErr.Category != kherrors.CategoryValidation {
			t.Errorf("got category %q, want validation", khErr.Category)
		}
		if khErr.ExitCode() != exitcodes.ValidationError {
			t.Errorf("got exit code %d, want %d", khErr.ExitCode(), exitcodes.ValidationError)
		}
	}
}
