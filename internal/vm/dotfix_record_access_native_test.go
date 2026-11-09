package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Record access is keyed to each captured record's synthetic owner, not the
// org-specific record ID. These are returned rows, including the None row.
func TestDotfixNativeRecordAccess(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/dotfix_fls_followups/org.tsv")
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
	checked := 0
	for _, row := range rows[1:] {
		if row[0] != "recordAccess" || row[2] != "test" || row[3] != "minimal" || (row[1] != "62.0" && row[1] != "67.0") {
			continue
		}
		var native struct {
			Inputs  []struct{ ID, Name string } `json:"inputOwners"`
			Records []struct {
				RecordID string `json:"RecordId"`
				Read     bool   `json:"HasReadAccess"`
				Edit     bool   `json:"HasEditAccess"`
				Max      string `json:"MaxAccessLevel"`
			} `json:"records"`
		}
		if err := json.Unmarshal([]byte(row[6]), &native); err != nil {
			t.Fatal(err)
		}
		ids := map[string]string{}
		for _, input := range native.Inputs {
			if input.Name == "Synthetic own" {
				ids[input.ID] = "a00000000000990"
			}
			if input.Name == "Synthetic other" {
				ids[input.ID] = "a00000000000991"
			}
		}
		source := "Id uid=UserInfo.getUserId(); Set<Id> ids=new Set<Id>{'a00000000000990','a00000000000991'}; List<UserRecordAccess> rows=[SELECT RecordId,HasReadAccess,HasEditAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId IN :ids WITH SYSTEM_MODE]; System.assertEquals(2,rows.size()); for(UserRecordAccess r:rows){"
		for _, record := range native.Records {
			id := ids[record.RecordID]
			if id == "" {
				t.Fatal("native record has no matching input")
			}
			source += fmt.Sprintf("if(r.RecordId=='%s'){System.assertEquals(%t,r.HasReadAccess);System.assertEquals(%t,r.HasEditAccess);System.assertEquals('%s',r.MaxAccessLevel);}", id, record.Read, record.Edit, record.Max)
		}
		source += "}"
		checked++
		t.Run(row[1], func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = dotfixRestrictedMachine(row[1]).ExecuteTestInClass(program, "SynthFlsCapture"); err != nil {
				t.Fatal(err)
			}
		})
	}
	if checked != 2 {
		t.Errorf("record access observations=%d, want 2", checked)
	}
}
