package server

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
)

// The shell document may differ from the pre-versioning document only in the
// two asset URLs, which gain a content version query.
func TestLWCShellDocumentOnlyAssetURLsChange(t *testing.T) {
	body := renderLWCShellDocument(project.Project{}, lwcbrowser.PageConfig{}, lwcshell.ShellPage{}, "/lwc/builder", "001000000000001AAA")
	versioned := regexp.MustCompile(`(/lightning/runtime/shell/(?:glade-shell\.css|app\.js))\?v=[0-9a-f]{16}`)
	if got := len(versioned.FindAllString(body, -1)); got != 2 {
		t.Fatalf("versioned asset URLs = %d, want 2", got)
	}
	plain := versioned.ReplaceAllString(body, "$1")
	if strings.Contains(plain, "?v=") {
		t.Fatal("unexpected version query outside the two shell assets")
	}
	const want = "839b65a47045c71c1321d65b1805ecf88249138e6d148d177d96e8d422a234ac"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(plain))); got != want {
		t.Fatalf("document digest without version queries = %s, want %s (origin/main digest)", got, want)
	}
}
