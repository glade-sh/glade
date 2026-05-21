package storage

import "testing"

func TestRuntimeTemplateSnapshotReadThroughAndObjectWriteIsolation(t *testing.T) {
	org := benchmarkOrgState(2, 2)
	account := org.Objects["PerfObject0__c"]
	account.Indexes = map[string]IndexSet{
		"Name": {
			Definition: IndexDefinition{Name: "Name", Fields: []string{"Name"}},
			Entries: map[string][]ID{
				"Acme": {"a00000000000000"},
			},
		},
	}
	org.Objects["PerfObject0__c"] = account
	template := NewRuntimeTemplate(org)
	clone := template.CloneSnapshotOrg()

	if !sameRecordMap(org.Objects["PerfObject0__c"].Records, clone.Objects["PerfObject0__c"].Records) {
		t.Fatalf("snapshot did not share record map before write")
	}
	object, cloned := EnsureMutableObjectRecords(&clone, "PerfObject0__c")
	if !cloned {
		t.Fatalf("object records were not cloned")
	}
	var id ID
	for recordID := range object.Records {
		id = recordID
		break
	}
	record := object.Records[id]
	record.Fields["Name"] = StringValue("Changed")
	object.Records[id] = record
	delete(object.Records, "a00000000000000")
	object.Indexes["Name"].Entries["Changed"] = []ID{id}
	clone.Objects["PerfObject0__c"] = *object

	if sameRecordMap(org.Objects["PerfObject0__c"].Records, clone.Objects["PerfObject0__c"].Records) {
		t.Fatalf("written object still shares record map")
	}
	if got := org.Objects["PerfObject0__c"].Records[id].Fields["Name"].String; got == "Changed" {
		t.Fatalf("snapshot write changed base record")
	}
	if _, ok := org.Objects["PerfObject0__c"].Records["a00000000000000"]; !ok {
		t.Fatalf("snapshot delete changed base record map")
	}
	if _, ok := org.Objects["PerfObject0__c"].Indexes["Name"].Entries["Changed"]; ok {
		t.Fatalf("snapshot index write changed base index map")
	}
	if !sameRecordMap(org.Objects["PerfObject1__c"].Records, clone.Objects["PerfObject1__c"].Records) {
		t.Fatalf("unwritten object record map was cloned")
	}
	clone = template.CloneSnapshotOrg()
	if got := clone.Objects["PerfObject0__c"].Records[id].Fields["Name"].String; got == "Changed" {
		t.Fatalf("fresh snapshot kept discarded overlay write")
	}
}

func TestRuntimeRollbackSnapshotMarksLiveAndSnapshotShared(t *testing.T) {
	org := benchmarkOrgState(1, 2)
	snapshot := SnapshotRuntimeOrg(&org)

	if !sameRecordMap(org.Objects["PerfObject0__c"].Records, snapshot.Objects["PerfObject0__c"].Records) {
		t.Fatalf("rollback snapshot did not share record map before write")
	}
	if !org.Objects["PerfObject0__c"].RecordsShared {
		t.Fatalf("live object was not marked shared")
	}
	if !snapshot.Objects["PerfObject0__c"].RecordsShared {
		t.Fatalf("snapshot object was not marked shared")
	}

	object, cloned := EnsureMutableObjectRecords(&org, "PerfObject0__c")
	if !cloned {
		t.Fatalf("live object records were not cloned on write")
	}
	var id ID
	for recordID := range object.Records {
		id = recordID
		break
	}
	record := object.Records[id]
	original := snapshot.Objects["PerfObject0__c"].Records[id].Fields["Name"].String
	record.Fields["Name"] = StringValue("Changed")
	object.Records[id] = record
	org.Objects["PerfObject0__c"] = *object

	if sameRecordMap(org.Objects["PerfObject0__c"].Records, snapshot.Objects["PerfObject0__c"].Records) {
		t.Fatalf("live object still shares record map after write")
	}
	if got := snapshot.Objects["PerfObject0__c"].Records[id].Fields["Name"].String; got != original {
		t.Fatalf("live write changed rollback snapshot: %q", got)
	}
}

func TestRuntimeTemplateSnapshotSequenceIsolation(t *testing.T) {
	org := benchmarkOrgState(1, 1)
	org.IDSequences["PerfObject0__c"] = 1
	clone := NewRuntimeTemplate(org).CloneSnapshotOrg()
	clone.IDSequences["PerfObject0__c"] = 2
	if got := org.IDSequences["PerfObject0__c"]; got != 1 {
		t.Fatalf("snapshot shared ID sequence map: %d", got)
	}
}
