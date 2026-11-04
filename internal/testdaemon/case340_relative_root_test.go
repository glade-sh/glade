package testdaemon

import (
	"os"
	"path/filepath"
	"testing"
)

// A relative project root that only climbs out of the working directory has no
// symlink ambiguity, so discovery paths for it must stay usable.
func TestCase340LeadingParentRelativeRootStaysUsable(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "a", "b")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	socket := ServeSocketPath(filepath.Join("..", "..", "proj"))
	if socketHasDotDot(socket) || !filepath.IsAbs(socket) {
		t.Fatalf("socket path keeps parent traversal: %q", socket)
	}
}
