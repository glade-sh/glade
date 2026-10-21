package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// JSON allocation must not read persisted fields into this sparse User value.
// This guards the candidate's transport boundary, not native User equality.
func TestJSONSparseUserDoesNotLoadStoredForeignKeys(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	user := org.Objects["User"]
	user.Records["005000000000001"] = storage.Record{Object: "User", ID: "005000000000001", Fields: map[string]storage.Value{
		"ProfileId":  storage.IDValue("00e000000000001"),
		"UserRoleId": storage.IDValue("00E000000000001"),
	}}
	org.Objects["User"] = user
	machine := New(nil)
	machine.SetOrg(&org)
	for _, target := range []string{"SObject", "User", "List<SObject>"} {
		t.Run(target, func(t *testing.T) {
			payload := map[string]any{"attributes": map[string]any{"type": "User"}, "Id": "005000000000001", "Username": "synthetic@example.invalid"}
			var input any = payload
			if target == "List<SObject>" {
				input = []any{payload}
			}
			got, err := machine.typedValueFromJSON(target, input, false)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind == ValueList {
				got = got.List[0]
			}
			for _, field := range []string{"ProfileId", "UserRoleId"} {
				if value, exists := got.Fields[field]; exists && value.Kind != ValueNull {
					t.Fatalf("JSON loaded absent %s: %#v", field, value)
				}
			}
			payload["ProfileId"] = "00e000000000001"
			explicit, err := machine.typedValueFromJSON("User", payload, false)
			if err != nil || explicit.Fields["ProfileId"].Kind == ValueNull {
				t.Fatalf("explicit lookup lost: %#v, %v", explicit, err)
			}
		})
	}
}
