package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows cover the current user's direct UserRecordAccess query for a
// Private custom record owned by a queue with a direct member.
func TestPrivateQueueMemberUserRecordAccess(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_custom_queue_access/org.tsv")
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
	if len(rows) != 3 {
		t.Fatalf("native rows=%d, want 2", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			var want map[string]int
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			machine := recordSharingControlMachine(t, "Private")
			machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
			for _, name := range []string{"Group", "GroupMember"} {
				storage.EnsureStandardObject(machine.Org, name)
			}
			queue := storage.ID("00G000000000001")
			groups := machine.Org.Objects["Group"]
			groups.Records = map[storage.ID]storage.Record{queue: {ID: queue, Object: "Group", Fields: map[string]storage.Value{"Type": storage.StringValue("Queue")}}}
			machine.Org.Objects["Group"] = groups
			members := machine.Org.Objects["GroupMember"]
			members.Records = map[storage.ID]storage.Record{"011000000000001": {ID: "011000000000001", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue(queue), "UserOrGroupId": storage.IDValue("005000000000002")}}}
			machine.Org.Objects["GroupMember"] = members
			objects := machine.Org.Objects["Widget__c"]
			ids := make([]string, 0, len(objects.Records))
			for id := range objects.Records {
				ids = append(ids, string(id))
			}
			sort.Strings(ids)
			if len(ids) != 2 {
				t.Fatalf("sharing fixture records=%d, want 2", len(ids))
			}
			for i, id := range ids {
				record := objects.Records[storage.ID(id)]
				record.System.OwnerID = "005000000000001"
				if i == 0 {
					record.System.OwnerID = queue
				}
				objects.Records[storage.ID(id)] = record
			}
			machine.Org.Objects["Widget__c"] = objects
			machine.SetOrg(machine.Org)
			for i, key := range []string{"fullQueueMemberAccess", "otherOwnAccess"} {
				expected, ok := want[key]
				if !ok {
					t.Fatalf("native observation missing %s: %v", key, want)
				}
				source := fmt.Sprintf("Id rid='%s'; UserRecordAccess a=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId=:rid]; System.assertEquals(%t,a.HasReadAccess);", ids[i], expected == 1)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("%s: %v", key, err)
				}
			}
		})
	}
}
