package visualforce

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// TestApexParseMemoKeepsPageNames requires page registration with the
// declaration memo to equal registration without it across an edit that
// changes a controller's members, a delete and a rename, and the memo to skip
// the parse of unchanged files.
func TestApexParseMemoKeepsPageNames(t *testing.T) {
	root := t.TempDir()
	writePageNamesWorkProject(t, root, 4, 4)
	classDir := filepath.Join(root, "force-app", "main", "default", "classes")
	load := func() project.Project {
		t.Helper()
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	type outcome struct {
		names    []string
		rejected int
	}
	register := func(memo bool) (outcome, uint64) {
		was := apexParseMemoEnabled.Load()
		apexParseMemoEnabled.Store(memo)
		defer apexParseMemoEnabled.Store(was)
		before := apexSourceParses.Load()
		names, rejected := PageNamesWithDiagnostics(load(), nil)
		return outcome{names: names, rejected: len(rejected)}, apexSourceParses.Load() - before
	}
	apexParseMemo.entries = nil
	defer func() { apexParseMemo.entries = nil }()
	steps := []struct {
		name  string
		apply func()
	}{
		{"baseline", func() {}},
		{"member edit", func() {
			path := filepath.Join(classDir, "WorkClass1.cls")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Page 1 binds {!name}; renaming the property rejects it.
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), "public String name", "public String title", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"delete", func() {
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				if err := os.Remove(filepath.Join(classDir, "WorkClass2"+suffix)); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"rename", func() {
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				if err := os.Rename(filepath.Join(classDir, "WorkClass3"+suffix), filepath.Join(classDir, "Moved"+suffix)); err != nil {
					t.Fatal(err)
				}
			}
		}},
	}
	var previous outcome
	for i, step := range steps {
		step.apply()
		want, _ := register(false)
		got, _ := register(true)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: memo %+v, direct %+v", step.name, got, want)
		}
		if i > 0 && reflect.DeepEqual(want, previous) && step.name == "member edit" {
			t.Fatalf("%s did not change page registration: %+v", step.name, want)
		}
		previous = want
		if _, parses := register(true); parses != 0 {
			t.Fatalf("%s: repeat registration parsed %d unchanged files", step.name, parses)
		}
	}
}
