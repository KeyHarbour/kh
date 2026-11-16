package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the package directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the package directory")
		}
		dir = parent
	}
}

// The release injects the version with
//
//	-X <module>/pkg/version.Version={{.Version}}
//
// and the Go linker silently ignores -X when the symbol path does not resolve:
// the build succeeds, no warning is printed, and every released binary reports
// "dev". Renaming the module without updating .goreleaser.yml is therefore a
// silent, user-visible regression that no build or test would otherwise catch.
//
// This pins the two together.
func TestGoreleaserLdflagMatchesModulePath(t *testing.T) {
	root := repoRoot(t)

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var module string
	for _, line := range strings.Split(string(gomod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			module = strings.TrimSpace(rest)
			break
		}
	}
	if module == "" {
		t.Fatal("no module directive found in go.mod")
	}

	releaser, err := os.ReadFile(filepath.Join(root, ".goreleaser.yml"))
	if err != nil {
		t.Skipf("no .goreleaser.yml at %s: %v", root, err)
	}

	want := "-X " + module + "/pkg/version.Version="
	if !strings.Contains(string(releaser), want) {
		var found string
		for _, line := range strings.Split(string(releaser), "\n") {
			if strings.Contains(line, "version.Version=") {
				found = strings.TrimSpace(line)
				break
			}
		}
		t.Errorf("goreleaser must inject the version through the current module path.\n"+
			"  go.mod module : %s\n"+
			"  expected      : %s{{.Version}}\n"+
			"  found         : %s\n"+
			"A mismatch links cleanly and ships binaries that report %q.",
			module, want, found, Version)
	}
}
