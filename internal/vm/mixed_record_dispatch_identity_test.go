package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Mixed token outcomes remain pending native rows. Both shared dispatch paths
// must agree on the same record identities, including explicit Schema aliases.
func TestMixedGenericRecordTokenDispatchAgrees(t *testing.T) {
	for _, test := range []struct {
		name    string
		records []Value
	}{
		{"ordinary", []Value{Object("Account"), Object("Contact")}},
		{"qualified", []Value{Object("Schema.Account"), Object("Schema.Contact")}},
		{"homogeneous-alias", []Value{Object("Account"), Object("Schema.Account")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			machine.SetOrg(&org)
			rows := List(test.records...)
			rows.Type = "List<SObject>"
			want, handled, err := machine.callListValueMember("rows", rows, "getSObjectType", nil, &Result{})
			if err != nil || !handled {
				t.Fatalf("VM dispatch: handled=%v error=%v", handled, err)
			}
			got, _, _, handled, err := callListStdlibMember(rows, "getSObjectType", nil)
			if err != nil || !handled {
				t.Fatalf("stdlib dispatch: handled=%v error=%v", handled, err)
			}
			if !got.Equal(want) {
				t.Fatalf("stdlib token=%s; VM token=%s", got.String(), want.String())
			}
		})
	}
}

// Source-class values stay outside the new record-only candidate. This preserves
// the pre-candidate stdlib boundary; it is not a native mixed-list observation.
func TestMixedRecordTokenKeepsSourceClassBoundary(t *testing.T) {
	for _, flag := range []string{"classInstance", "projectClass"} {
		for _, position := range []int{0, 1, 2} {
			t.Run(flag+string(rune('0'+position)), func(t *testing.T) {
				source := Object("Parcel")
				if flag == "classInstance" {
					source.classInstance = true
				} else {
					source.projectClass = true
				}
				items := []Value{Object("Account"), Object("Contact")}
				items = append(items, Null)
				copy(items[position+1:], items[position:])
				items[position] = source
				rows := List(items...)
				rows.Type = "List<SObject>"
				got, _, _, handled, err := callListStdlibMember(rows, "getSObjectType", nil)
				if err != nil || !handled || !got.Equal(sObjectTypeToken("Account")) {
					t.Fatalf("source-class boundary token=%s; handled=%v; error=%v", got.String(), handled, err)
				}
			})
		}
	}
}
