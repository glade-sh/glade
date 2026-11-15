package startupcache

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// A warm test run must see the same source spread visibility as the cold run
// that wrote the cache: untouched inventory additions stay hidden and edited
// ones stay visible.
func TestSplitPayloadPreservesPlainSpreadSuppression(t *testing.T) {
	dir := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(dir, "sfdx-project.json"), `{"packageDirectories":[]}`)
	p, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.OrgID = storage.DefaultOrgID
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	for _, name := range []string{"CareProgram", "CareProgramEnrollee"} {
		if _, ok := storage.EnsureLazyStandardObject(&org, name); !ok {
			t.Fatalf("inventory object %s was not materialized", name)
		}
	}
	edited := org.Objects["CareProgramEnrollee"]
	edited.Definition.Label = "edited"
	org.Objects["CareProgramEnrollee"] = edited
	want := map[string]bool{"CareProgram": true, "CareProgramEnrollee": false}
	for name, hidden := range want {
		if got := org.SuppressesSpreadObject(name); got != hidden {
			t.Fatalf("cold %s suppressed = %t, want %t", name, got, hidden)
		}
	}
	entry := NewEntry(dir, p, typesys.Index{}, org, CompiledRuntime{})
	for _, subdir := range []string{SubdirTest, SubdirDAP} {
		if err := Write(&entry, subdir); err != nil {
			t.Fatalf("%s Write() error = %v", subdir, err)
		}
		got, err := Read(dir, subdir)
		if err != nil || got == nil {
			t.Fatalf("%s Read() = %v, %v", subdir, got, err)
		}
		if !got.Org.StandardSpreadCapturedPlain {
			t.Fatalf("%s warm org lost the captured shape", subdir)
		}
		for name, hidden := range want {
			if suppressed := got.Org.SuppressesSpreadObject(name); suppressed != hidden {
				t.Errorf("%s warm %s suppressed = %t, want %t", subdir, name, suppressed, hidden)
			}
		}
	}
	for name, hidden := range want {
		if got := org.SuppressesSpreadObject(name); got != hidden {
			t.Errorf("writing the cache changed cold %s suppressed to %t", name, got)
		}
	}
}
