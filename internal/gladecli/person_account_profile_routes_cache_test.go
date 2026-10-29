package gladecli

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileRoutesRejectLegacyDAPCache(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_profile_routes"
	const legacyVersion = 10
	const staleMarker = "legacy-business-profile-default"
	t.Setenv("GLADE_HOME", t.TempDir())
	root := copyProfileRoutesLoaderProject(t, fixture, "67.0", true)
	if _, _, _, err := loadDAPStartupState(root); err != nil {
		t.Fatal(err)
	}
	entry, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("DAP cache was not populated")
	}
	profileID := storage.ID("")
	for id, record := range entry.Org.Objects["Profile"].Records {
		if record.Fields["Name"].String == "Route False" {
			profileID = id
		}
	}
	if profileID == "" {
		t.Fatal("captured Route False profile missing")
	}
	want, wrong := profileRoutesDAPCacheNativeDefaults(t, fixture)
	if want == wrong {
		t.Fatal("fixture does not distinguish profile business defaults")
	}
	mappings := entry.Org.Metadata.AccountRecordTypeDefaults[:0]
	for _, mapping := range entry.Org.Metadata.AccountRecordTypeDefaults {
		if !storage.IDsEqual(mapping.ProfileID, profileID) || mapping.PersonAccount {
			mappings = append(mappings, mapping)
		}
	}
	entry.Org.Metadata.AccountRecordTypeDefaults = append(mappings, storage.AccountRecordTypeDefault{ProfileID: profileID, DeveloperName: wrong})
	entry.Version, entry.BuiltAt = legacyVersion, staleMarker
	if !startupcache.Fresh(entry, root, legacyVersion) {
		t.Fatal("legacy DAP cache fixture is not otherwise fresh")
	}
	if err := startupcache.Write(entry, startupcache.SubdirDAP); err != nil {
		t.Fatal(err)
	}
	org, _, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	users := org.Objects["User"]
	user := users.Records[storage.DefaultUserID]
	user.Fields["ProfileId"] = storage.IDValue(profileID)
	users.Records[storage.DefaultUserID] = user
	org.Objects["User"] = users
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}
	selected, ok := dml.AccountDefaultRecordType(&org, storage.DefaultUserID, org.Objects["Account"].Definition, record)
	if !ok || selected.DeveloperName != want {
		t.Fatalf("restored business default: got %s (found=%v), native %s", selected.DeveloperName, ok, want)
	}
	rebuilt, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt == nil || rebuilt.Version == legacyVersion || rebuilt.Version != dapCacheVersion || rebuilt.BuiltAt == staleMarker {
		t.Fatal("legacy DAP cache was not replaced with the current version")
	}
}

func profileRoutesDAPCacheNativeDefaults(t *testing.T, fixture string) (string, string) {
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
	native := map[string]string{}
	for _, row := range rows {
		if row[0] != "67.0" || row[1] != "istest" || (row[2] != "false_scalar_business_omitted" && row[2] != "admin_scalar_business_omitted") {
			continue
		}
		if row[4] != "false" || native[row[2]] != "" {
			t.Fatalf("invalid native DAP cache row %v", row)
		}
		native[row[2]] = row[3]
	}
	if len(native) != 2 {
		t.Fatalf("native DAP cache rows=%d, want 2", len(native))
	}
	return native["false_scalar_business_omitted"], native["admin_scalar_business_omitted"]
}
