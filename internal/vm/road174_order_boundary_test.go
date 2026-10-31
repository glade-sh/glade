package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRoad174OrderBoundary(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_access_matrix/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows[1:] {
		if r[1] != "istest" || r[2] != "Order" || r[3] != "-" || r[4] != "admin" || r[6] != "relationship" {
			continue
		}
		row := r
		t.Run(row[0]+"/"+row[5], func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[7]), &want); err != nil || len(want) != 6 {
				t.Fatalf("bad row: %v %v", want, err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Order")
			org.APIVersion = row[0]
			id := storage.ID("801000000000001")
			object := org.Objects["Order"]
			object.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Order", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}}
			org.Objects["Order"] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "Road174OrderBoundary", APIVersion: row[0], Modifiers: []string{"isTest"}}); err != nil {
				t.Fatal(err)
			}
			fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
			var src strings.Builder
			if row[5] == "runas" {
				src.WriteString("System.runAs(new User(Id=UserInfo.getUserId())){")
			}
			fmt.Fprintf(&src, "List<SObject> rows=Database.query('SELECT %s FROM Order WHERE Id=\\'%s\\'');System.assertEquals(1,rows.size());SObject access=rows[0].getSObject('UserRecordAccess');", "UserRecordAccess."+strings.Join(fields, ",UserRecordAccess."), id)
			for i, field := range fields {
				var value string
				if b, ok := want[i].(bool); ok {
					value = fmt.Sprint(b)
				} else {
					value = fmt.Sprintf("'%s'", want[i])
				}
				fmt.Fprintf(&src, "System.assertEquals(%s,access.get('%s'));", value, field)
			}
			if row[5] == "runas" {
				src.WriteString("}")
			}
			prog, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(prog, "Road174OrderBoundary"); err != nil {
				t.Fatalf("native %s: %v", row[7], err)
			}
		})
	}
}
