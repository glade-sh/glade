package packageartifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSourceHashTracksApprovalProcessMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Account.Review.approvalProcess-meta.xml")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApprovalProcessFiles: []string{path}}
	first, err := SourceHash(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := SourceHash(p)
	if err != nil {
		t.Fatal(err)
	}
	if first == next {
		t.Fatal("approval metadata edit did not change source hash")
	}
}
