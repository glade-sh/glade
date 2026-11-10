package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native with-sharing queries exercise the same record-visibility helper
// used by the runtime query paths. Retrieval shows both phases remain public.
func TestFlowFollowupsCasePublicSharing(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_case_sharing/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	const (
		a     storage.ID = "005000000000001"
		b     storage.ID = "005000000000002"
		empty storage.ID = "00G000000000001"
		full  storage.ID = "00G000000000002"
	)
	models := map[string]string{"unmodified": "ReadWriteTransfer", "private": "ReadWriteTransfer"}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			var want map[string]int
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != 9 {
				t.Fatalf("native setup did not produce nine observations: %v", want)
			}
			org := storage.NewOrgState()
			org.APIVersion = row[1]
			for _, name := range []string{"Case", "Group", "GroupMember"} {
				storage.EnsureStandardObject(&org, name)
			}
			leads := org.Objects["Case"]
			leads.Definition.SharingModel = models[row[0]]
			org.Objects["Case"] = leads
			groups := org.Objects["Group"]
			groups.Records = map[storage.ID]storage.Record{}
			for _, id := range []storage.ID{empty, full} {
				groups.Records[id] = storage.Record{ID: id, Object: "Group", Fields: map[string]storage.Value{"Type": storage.StringValue("Queue")}}
			}
			org.Objects["Group"] = groups
			members := org.Objects["GroupMember"]
			members.Records = map[storage.ID]storage.Record{"011000000000001": {ID: "011000000000001", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.StringValue(string(full)), "UserOrGroupId": storage.StringValue(string(b))}}}
			org.Objects["GroupMember"] = members
			machine := New(nil)
			machine.SetOrg(&org)
			for _, tc := range []struct {
				name         string
				actor, owner storage.ID
			}{
				{"owner", a, a}, {"creatorEmptyQueue", a, empty}, {"creatorFullQueue", a, full},
				{"otherOwn", b, a}, {"otherEmptyQueue", b, empty}, {"fullQueueMember", b, full},
				{"creatorFullQueueUser", a, full}, {"fullQueueMemberUser", b, full}, {"otherOwnUser", b, a},
			} {
				machine.SetCurrentUser(storage.Record{ID: tc.actor, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("Standard")}})
				record := storage.Record{ID: "500000000000001", Object: "Case", System: storage.SystemFields{OwnerID: tc.owner, CreatedByID: a}}
				got := 0
				if machine.userModeRecordVisible("Case", record, string(tc.actor)) {
					got = 1
				}
				if got != want[tc.name] {
					t.Errorf("%s: visibility=%d, native=%d", tc.name, got, want[tc.name])
				}
			}
		})
	}
	if len(rows) != 5 {
		t.Fatalf("native rows=%d, want 4", len(rows)-1)
	}
}
