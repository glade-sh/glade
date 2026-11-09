package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The anonymous API 62 rows are repeated at API 67 and on the test route.
// IDs differ by org, so their equality relationships are compared instead.
func TestPermissionSetInventoryNativeRows(t *testing.T) {
	f, err := os.Open("testdata/conformance/permission_set_inventory/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	sets := map[string]storage.Record{}
	for _, record := range org.Objects["PermissionSet"].Records {
		if !record.Fields["IsOwnedByProfile"].Boolean {
			sets[record.Fields["Name"].String] = record
		}
	}
	licenses := map[string]storage.ID{}
	licenseOwners := map[storage.ID]string{}
	count := 0
	for _, row := range rows[1:] {
		if row[2] != "62.0" || row[3] != "anonymous" || row[5] != "queryRow" {
			continue
		}
		var native struct {
			Query  string `json:"queryId"`
			Record struct {
				Name      string `json:"Name"`
				Label     string `json:"Label"`
				Type      string `json:"Type"`
				Namespace string `json:"NamespacePrefix"`
				License   string `json:"LicenseId"`
				Owned     bool   `json:"IsOwnedByProfile"`
				Custom    bool   `json:"IsCustom"`
				APIOnly   bool   `json:"PermissionsApiUserOnly"`
				ModifyAll bool   `json:"PermissionsModifyAllData"`
				ViewAll   bool   `json:"PermissionsViewAllData"`
			} `json:"record"`
		}
		if err := json.Unmarshal([]byte(row[8]), &native); err != nil {
			t.Fatal(err)
		}
		want := native.Record
		if native.Query != "all" || want.Owned {
			continue
		}
		count++
		got, ok := sets[want.Name]
		if !ok {
			t.Errorf("native permission set %s missing", want.Name)
			continue
		}
		for key, value := range map[string]string{"Label": want.Label, "Type": want.Type, "NamespacePrefix": want.Namespace} {
			if got.Fields[key].String != value {
				t.Errorf("%s.%s = %q, native %q", want.Name, key, got.Fields[key].String, value)
			}
		}
		for key, value := range map[string]bool{"IsCustom": want.Custom, "IsOwnedByProfile": want.Owned, "PermissionsApiUserOnly": want.APIOnly, "PermissionsModifyAllData": want.ModifyAll, "PermissionsViewAllData": want.ViewAll} {
			if got.Fields[key].Boolean != value {
				t.Errorf("%s.%s = %t, native %t", want.Name, key, got.Fields[key].Boolean, value)
			}
		}
		id := got.Fields["LicenseId"].ID
		if want.License != "" && id == "" {
			t.Errorf("%s has no license", want.Name)
			continue
		}
		if previous, ok := licenses[want.License]; ok && previous != id {
			t.Errorf("%s: shared native license has different local IDs", want.Name)
		}
		if previous, ok := licenseOwners[id]; ok && previous != want.License {
			t.Errorf("%s: distinct native licenses share a local ID", want.Name)
		}
		licenses[want.License], licenseOwners[id] = id, want.License
		if len(got.ID) < 15 || got.ID[:6] == "0PS000" {
			t.Errorf("%s has counter-style seeded ID %s", want.Name, got.ID)
		}
	}
	if count != 50 || len(sets) != count {
		t.Errorf("non-profile sets: native %d, Glade %d", count, len(sets))
	}
}
