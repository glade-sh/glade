package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// The native rows (internal/apextest/testdata/conformance/flow_decision_shapes/
// org.tsv PARENT:Open and PARENT:Open3, API 62.0 and 67.0, anonymous and
// istest, dt=set) observe a system-only $Record.LastModifiedDate as an Update
// Records assignment only when the element targets the triggering record's
// parent through inputReference $Record.<Relationship>__r. An Update Records
// element that selects its target by filter has no native row, so its
// assignment keeps the base (a recorded revision) null resolution.
func TestFlowAuditSystemTimestampUpdateRecordsTargetPaths(t *testing.T) {
	const stamp = "2026-10-19T01:02:03.000Z"
	const parentID = storage.ID("001000000000001")
	for _, api := range []string{"62.0", "67.0"} {
		for _, path := range []struct {
			name   string
			update storage.FlowRecordUpdate
			want   storage.Value
		}{
			{name: "parent_relationship", update: storage.FlowRecordUpdate{Name: "UpdParent", ObjectName: "Account", InputReference: "$Record.Parent_Acct__r"}, want: storage.DateTimeValue(stamp)},
			{name: "filter_selected", update: storage.FlowRecordUpdate{Name: "UpdFiltered", ObjectName: "Account", Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "$Record.Parent_Acct__c"}}}, want: storage.NullValue()},
		} {
			t.Run(api+"/"+path.name, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = api
				account := org.Objects["Account"]
				account.Definition.Fields["Observed__c"] = storage.Field{APIName: "Observed__c", Type: storage.FieldDateTime}
				account.Records[parentID] = storage.Record{Object: "Account", ID: parentID, Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}}
				org.Objects["Account"] = account
				engine := NewEngine(&org)
				definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
					"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime},
					"Parent_Acct__c":   {APIName: "Parent_Acct__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent_Acct__r"},
				}}
				source := storage.Record{Object: "Contact", ID: "003000000000001", Fields: map[string]storage.Value{"Parent_Acct__c": storage.IDValue(parentID)}, System: storage.SystemFields{LastModifiedDate: stamp}}
				frame := newFlowFrame()
				frame.saveDecisions = true
				seedFlowFrame(&engine, &source, frame)
				update := path.update
				update.InputAssignments = []storage.WorkflowFieldUpdate{{Field: "Observed__c", SourceField: "$Record.LastModifiedDate"}}
				count, err := engine.executeFlowRecordUpdate(update, source, definition, frame)
				if err != nil || count != 1 {
					t.Fatalf("updated=%d err=%v; want one parent update", count, err)
				}
				got, ok := org.Objects["Account"].Records[parentID].Fields["Observed__c"]
				if !ok {
					got = storage.NullValue()
				}
				// Storage normalizes a saved DateTime ("...03.000Z" -> "...03Z"),
				// so compare the instant rather than the spelling.
				same := got.Kind == path.want.Kind
				if same && got.Kind == storage.ValueDateTime {
					left, leftErr := time.Parse(time.RFC3339Nano, got.String)
					right, rightErr := time.Parse(time.RFC3339Nano, path.want.String)
					same = leftErr == nil && rightErr == nil && left.Equal(right)
				}
				if !same {
					t.Fatalf("Observed__c=%#v; want %#v", got, path.want)
				}
			})
		}
	}
}
