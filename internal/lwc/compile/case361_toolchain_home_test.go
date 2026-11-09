//go:build darwin || linux

package compile

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCase361CompileReportsMissingToolchainHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Setenv("TMPDIR", t.TempDir())
	_, err := Compile(project.Project{}, Options{OutDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Fatalf("expected missing HOME diagnostic, got %v", err)
	}
}
