package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileMetadataInvalidatesWarmRuntime(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_profile_defaults"
	root := t.TempDir()
	err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		if rel != "sfdx-project.json" && !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
			return nil
		}
		writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(readFixtureFile(t, path), "67.0", "62.0"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(InvalidateRuntimeCaches)
	index := loadTestIndex(t, root)
	sources := newSourceCache()
	if _, err := prepareRuntimeGeneration(index, nil, sources); err != nil {
		t.Fatal(err)
	}
	beforeKey, before, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	assertPersonAccountProfileSelection(t, before.restored.CloneOrg(), "Provider", "Business_Default")

	// Both configurations are captured in the API 62 fixture. Only the profile
	// bytes change; Apex and the schema index remain the same.
	profiles := filepath.Join(root, "force-app/main/default/profiles")
	writeFile(t, filepath.Join(profiles, "Admin.profile-meta.xml"), readFixtureFile(t, filepath.Join(profiles, "Default Generic.profile-meta.xml")))
	captured, err := orgFromIndexWithError(index, sources)
	if err != nil {
		t.Fatal(err)
	}
	assertPersonAccountProfileSelection(t, captured, "Provider", "Business_Default")
	if err := sources.validateCapturedSourceGeneration(); err == nil {
		t.Fatal("profile drift did not invalidate the captured generation")
	}
	afterKey, after, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if beforeKey == afterKey {
		t.Fatal("profile-only edit reused the previous runtime key")
	}
	assertPersonAccountProfileSelection(t, after.restored.CloneOrg(), "PersonAccount", "Business_Alternate")
}

func assertPersonAccountProfileSelection(t *testing.T, org storage.OrgState, person, business string) {
	t.Helper()
	definition := org.Objects["Account"].Definition
	for _, row := range []struct {
		field string
		want  string
	}{{"LastName", person}, {"Name", business}} {
		record := storage.Record{Object: "Account", Fields: map[string]storage.Value{row.field: storage.StringValue("Probe")}}
		rt, ok := dml.AccountDefaultRecordType(&org, storage.DefaultUserID, definition, record)
		if !ok || rt.DeveloperName != row.want {
			t.Fatalf("%s default: got %s (found=%v), want %s", row.field, rt.DeveloperName, ok, row.want)
		}
	}
}
