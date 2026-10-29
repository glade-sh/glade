package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileRoutesRejectLegacyRuntimeCache(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_profile_routes"
	const legacyABI = "apextest-runtime-v11"
	const staleMarker = "legacy-business-profile-default"
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
			return nil
		}
		writeFile(t, filepath.Join(root, rel), readFixtureFile(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	index := loadTestIndex(t, root)
	key, built, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	org := built.restored.CloneOrg()
	profileID := storage.ID("")
	for id, record := range org.Objects["Profile"].Records {
		if record.Fields["Name"].String == "Route False" {
			profileID = id
		}
	}
	if profileID == "" {
		t.Fatal("captured Route False profile missing")
	}
	native := profileRoutesCacheNativeDefaults(t, fixture)
	wantBusiness, wrongBusiness := native["false_scalar_business_omitted"], native["admin_scalar_business_omitted"]
	if wantBusiness == wrongBusiness {
		t.Fatal("fixture does not distinguish profile business defaults")
	}
	mappings := org.Metadata.AccountRecordTypeDefaults[:0]
	for _, mapping := range org.Metadata.AccountRecordTypeDefaults {
		if !storage.IDsEqual(mapping.ProfileID, profileID) || mapping.PersonAccount {
			mappings = append(mappings, mapping)
		}
	}
	org.Metadata.AccountRecordTypeDefaults = append(mappings, storage.AccountRecordTypeDefault{ProfileID: profileID, DeveloperName: wrongBusiness})
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := startupcache.NewEntry(root, p, index, org, startupcache.CompiledRuntime{
		Methods: built.Methods, Classes: built.Classes, Triggers: built.Triggers, PageNames: built.PageNames,
	})
	entry.RuntimeABI, entry.RuntimeKey, entry.BuiltAt = legacyABI, string(key), staleMarker
	if !startupcache.Fresh(&entry, root, startupcache.Version) {
		t.Fatal("legacy cache fixture is not otherwise fresh")
	}
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatal(err)
	}
	InvalidateRuntimeCaches()
	_, rebuilt, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true)
	if err != nil {
		t.Fatal(err)
	}
	org = rebuilt.restored.CloneOrg()
	users := org.Objects["User"]
	user := users.Records[storage.DefaultUserID]
	user.Fields["ProfileId"] = storage.IDValue(profileID)
	users.Records[storage.DefaultUserID] = user
	org.Objects["User"] = users
	assertPersonAccountProfileSelection(t, org, native["false_scalar_person_omitted"], wantBusiness)
	persisted, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.RuntimeABI == legacyABI || persisted.RuntimeABI != testRuntimeCacheABI || persisted.BuiltAt == staleMarker {
		t.Fatal("legacy runtime cache was not replaced with the current runtime")
	}
}

func profileRoutesCacheNativeDefaults(t *testing.T, fixture string) map[string]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(body)))
	reader.Comma, reader.FieldsPerRecord = '\t', 5
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]string{"false_scalar_business_omitted": "false", "admin_scalar_business_omitted": "false", "false_scalar_person_omitted": "true"}
	native := map[string]string{}
	for _, row := range rows {
		kind, ok := wanted[row[2]]
		if row[0] != "67.0" || row[1] != "istest" || !ok {
			continue
		}
		if row[4] != kind || native[row[2]] != "" {
			t.Fatalf("invalid native cache row %v", row)
		}
		native[row[2]] = row[3]
	}
	if len(native) != len(wanted) {
		t.Fatalf("native cache rows=%d, want %d", len(native), len(wanted))
	}
	return native
}
