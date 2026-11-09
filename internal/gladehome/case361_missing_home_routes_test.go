//go:build darwin || linux

package gladehome

import (
	"strings"
	"testing"
)

func TestCase361MissingHomeManagedRoutes(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Setenv("TMPDIR", t.TempDir())
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"lease", func() error {
			_, release, err := LeaseRoot()
			if release != nil {
				release()
			}
			return err
		}},
		{"ensure lease", func() error {
			_, release, err := LeaseEnsuredRoot()
			if release != nil {
				release()
			}
			return err
		}},
		{"install", func() error { return InstallFrom(t.TempDir()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "HOME") {
				t.Fatalf("expected missing HOME diagnostic, got %v", err)
			}
		})
	}
	if _, ok, detail := ToolchainStatus(); ok || !strings.Contains(detail, "HOME") {
		t.Fatalf("status ok=%v detail=%q", ok, detail)
	}
}
