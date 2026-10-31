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

// Replay owned administrator observations; values do not assert a general grant.
func TestRoad174ReviewPopulatedProjection(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/road174_review_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[1] != "anonymous" || row[2] != "projection" {
			continue
		}
		row := row
		t.Run(row[0]+"/"+row[3], func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[5]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != 6 {
				t.Fatal("incomplete row")
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, row[3])
			org.APIVersion = row[0]
			object := org.Objects[row[3]]
			id := storage.ID(object.Definition.KeyPrefix + "000000000001")
			object.Records = map[storage.ID]storage.Record{id: {ID: id, Object: row[3], Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}}
			org.Objects[row[3]] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
			machine.EnableTestContext()
			fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
			var src strings.Builder
			fmt.Fprintf(&src, "List<SObject> rows=Database.query('SELECT %s FROM %s WHERE Id=\\'%s\\'');System.assertEquals(1,rows.size());SObject access=rows[0].getSObject('UserRecordAccess');", "UserRecordAccess."+strings.Join(fields, ",UserRecordAccess."), row[3], id)
			for i, field := range fields {
				var value string
				if b, ok := want[i].(bool); ok {
					value = fmt.Sprint(b)
				} else {
					value = fmt.Sprintf("'%s'", want[i])
				}
				fmt.Fprintf(&src, "System.assertEquals(%s,access.get('%s'));", value, field)
			}
			program, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatalf("native %s %s: %v", row[3], row[5], err)
			}
		})
	}
}
