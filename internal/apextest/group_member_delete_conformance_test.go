package apextest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The native row permits deleting an inserted membership while retaining the
// setup-insert restriction on subsequent Account work.
func TestGroupMemberDeleteOrgConformance(t *testing.T) {
	const file = "group_member_delete.json"
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Rows []nativeBoundaryRow }
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 1 || data.Rows[0].Expected == nil {
		t.Fatal("missing native membership delete row")
	}
	rows, err := os.ReadFile("testdata/conformance/group_member_delete/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(rows)), "\n")
	if len(lines) != 4 || lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("incomplete native membership delete matrix")
	}
	for i, api := range []string{"62.0", "67.0", "68.0"} {
		if lines[i+1] != api+"\tisTest\tMD01T\t"+data.Rows[0].Expected.Value {
			t.Fatal("native membership delete binding changed", api)
		}
	}
	runNativeBoundaryConformance(t, file, "GroupMemberDelete", 1)
}
