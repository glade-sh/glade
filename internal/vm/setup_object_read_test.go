package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSetupObjectReadSharingNativeRows(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/setup_object_read/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native matrix: %v, rows=%d", err, len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] != "isTest" {
			continue
		}
		var observed map[string]any
		if err := json.Unmarshal([]byte(row[2]), &observed); err != nil {
			t.Fatal(err)
		}
		for _, actor := range []string{"Standard User", "Minimum Access - Salesforce"} {
			for object, key := range map[string]string{
				"UserRole": "roles.count", "Profile": "profile", "RecordType": "recordtype",
				"CronTrigger": "schedule", "CronJobDetail": "schedule.detail", "AsyncApexJob": "job",
				"PermissionSetAssignment": "assignment",
			} {
				t.Run(row[0]+"/runAs/"+actor+"/"+object, func(t *testing.T) {
					count, ok := observed[actor+"."+key].(float64)
					if !ok || count <= 0 {
						t.Fatalf("missing positive native read count for %s", key)
					}
					machine, record := defaultSharingWriteFixture(object, "", false)
					if !machine.userModeRecordVisible(object, record, machine.currentUserID()) {
						t.Fatal("creator ownership hid a natively readable setup row")
					}
					// A read grant must not create write authority.
					for _, operation := range []string{"update", "upsert", "delete", "undelete"} {
						if machine.currentUserCanWriteRecord(object, record, machine.currentUserID(), operation) {
							t.Fatalf("setup read granted another owner's %s", operation)
						}
					}
				})
			}
		}
	}
}

func TestSetupObjectReadPreservesPrivateAndAssignmentRoutes(t *testing.T) {
	for _, object := range []string{"MeadowEntry__c", "PermissionSetAssignment", "PermissionSetGroup", "QueueSobject"} {
		t.Run(object+"/otherUser", func(t *testing.T) {
			machine, record := defaultSharingWriteFixture(object, "Private", false)
			record.Fields["AssigneeId"] = storage.IDValue(record.System.OwnerID)
			if machine.userModeRecordVisible(object, record, machine.currentUserID()) {
				t.Fatal("setup read escaped into a private or assignment-scoped object")
			}
			if strings.EqualFold(object, "PermissionSetAssignment") {
				record.Fields["AssigneeId"] = storage.IDValue(storage.ID(machine.currentUserID()))
				if !machine.userModeRecordVisible(object, record, machine.currentUserID()) {
					t.Fatal("own permission assignment became hidden")
				}
			}
		})
	}
}

func TestSetupObjectReadSiblingNativeRows(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/setup_object_direct_read/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native sibling matrix: %v, rows=%d", err, len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] != "isTest" {
			continue
		}
		var observed map[string]any
		if err := json.Unmarshal([]byte(row[2]), &observed); err != nil {
			t.Fatal(err)
		}
		for object, key := range map[string]string{"PermissionSetGroup": "bundle", "QueueSobject": "queue", "GroupMember": "member"} {
			t.Run(row[0]+"/runAs/StandardUser/"+object, func(t *testing.T) {
				if observed["Standard User."+key] != float64(1) {
					t.Fatalf("missing native sibling visibility: %s", key)
				}
				machine, record := defaultSharingWriteFixture(object, "", false)
				if !machine.userModeRecordVisible(object, record, machine.currentUserID()) {
					t.Fatal("creator ownership hid a natively readable setup sibling")
				}
				for _, operation := range []string{"update", "upsert", "delete", "undelete"} {
					if machine.currentUserCanWriteRecord(object, record, machine.currentUserID(), operation) {
						t.Fatalf("setup sibling read granted another owner's %s", operation)
					}
				}
			})
		}
	}
}
