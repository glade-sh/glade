package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type permissionProfileFlags struct {
	Type, Namespace                     string
	Custom, APIOnly, ModifyAll, ViewAll bool
}

// Profile names and labels can incorporate org-specific IDs. Compare the
// complete multiset of observable flags without treating those IDs as stable.
func TestPermissionSetInventoryNativeProfileFlags(t *testing.T) {
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
	want := map[permissionProfileFlags]int{}
	for _, row := range rows[1:] {
		if row[2] != "62.0" || row[3] != "anonymous" || row[5] != "queryRow" {
			continue
		}
		var v struct {
			Query  string `json:"queryId"`
			Record struct {
				Type      string `json:"Type"`
				Namespace string `json:"NamespacePrefix"`
				Owned     bool   `json:"IsOwnedByProfile"`
				Custom    bool   `json:"IsCustom"`
				API       bool   `json:"PermissionsApiUserOnly"`
				Modify    bool   `json:"PermissionsModifyAllData"`
				View      bool   `json:"PermissionsViewAllData"`
			} `json:"record"`
		}
		if err := json.Unmarshal([]byte(row[8]), &v); err != nil {
			t.Fatal(err)
		}
		if v.Query != "all" || !v.Record.Owned {
			continue
		}
		x := v.Record
		want[permissionProfileFlags{x.Type, x.Namespace, x.Custom, x.API, x.Modify, x.View}]++
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	got := map[permissionProfileFlags]int{}
	for _, set := range org.Objects["PermissionSet"].Records {
		if !set.Fields["IsOwnedByProfile"].Boolean {
			continue
		}
		f := set.Fields
		got[permissionProfileFlags{f["Type"].String, f["NamespacePrefix"].String, f["IsCustom"].Boolean, f["PermissionsApiUserOnly"].Boolean, f["PermissionsModifyAllData"].Boolean, f["PermissionsViewAllData"].Boolean}]++
		if len(set.ID) < 15 || set.ID[:6] == "0PS000" {
			t.Errorf("counter-style profile set ID %s", set.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("profile flag groups: native=%v Glade=%v", want, got)
	}
}
