package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestFrontendPriorDigestIdentityAndReadErrors(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "First.cls"), filepath.Join(root, "Second.cls")
	writeFile(t, a, "public class First { Integer x; }")
	writeFile(t, b, "public class Second { Integer y; }")
	p := project.Project{Root: root, ApexFiles: []string{a, b}}
	_, artifacts := BuildWithArtifacts(p, schema.Schema{})
	before, err := frontendInputIdentity(p, schema.Schema{}, artifacts.SourceDigests)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, a, "public class First { Integer z; }")
	after, err := frontendInputIdentity(p, schema.Schema{}, artifacts.SourceDigests)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("prior digest identity changed with source bytes")
	}
	current, err := FrontendInputIdentity(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	if current == before {
		t.Fatal("ordinary identity ignored source edit")
	}
	writeFile(t, b+"-meta.xml", "<ApexClass/>")
	metadata, err := frontendInputIdentity(p, schema.Schema{}, artifacts.SourceDigests)
	if err != nil {
		t.Fatal(err)
	}
	if metadata == before {
		t.Fatal("sidecar change did not change prior identity")
	}
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if _, err := frontendInputIdentity(p, schema.Schema{}, artifacts.SourceDigests); err == nil {
		t.Fatal("missing source accepted")
	}
}
