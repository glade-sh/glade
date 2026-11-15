package startupcache

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// A warm test run must see the same source spread visibility as the cold run
// that wrote the cache: untouched inventory additions stay hidden and edited
// ones stay visible.
func TestSplitPayloadPreservesPlainSpreadSuppression(t *testing.T) {
	dir := t.TempDir()
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
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}, Org: org}
	if _, err := WriteWithStats(&entry, SubdirTest); err != nil {
		t.Fatalf("WriteWithStats() error = %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil || got == nil {
		t.Fatalf("Read() = %v, %v", got, err)
	}
	if !got.Org.StandardSpreadCapturedPlain {
		t.Fatal("warm org lost the captured shape")
	}
	for name, hidden := range want {
		if suppressed := got.Org.SuppressesSpreadObject(name); suppressed != hidden {
			t.Errorf("warm %s suppressed = %t, want %t", name, suppressed, hidden)
		}
	}
}
