package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestOwnerShareDeleteDMLRows(t *testing.T) {
	for _, test := range []struct {
		name, source string
	}{
		{"lifecycle", `
LedgerItem__c row=new LedgerItem__c(Name='Lifecycle');
insert row;
System.assertEquals(1,Limits.getDmlRows());
System.assertEquals(1,[SELECT count() FROM LedgerItem__Share WHERE ParentId=:row.Id]);
delete row;
System.assertEquals(2,Limits.getDmlStatements());
System.assertEquals(2,Limits.getDmlRows());
System.assertEquals(0,[SELECT count() FROM LedgerItem__Share WHERE ParentId=:row.Id]);
undelete row;
System.assertEquals(3,Limits.getDmlStatements());
System.assertEquals(3,Limits.getDmlRows());
System.assertEquals(1,[SELECT count() FROM LedgerItem__Share WHERE ParentId=:row.Id]);
`},
		{"partialDelete", `
List<LedgerItem__c> rows=new List<LedgerItem__c>{new LedgerItem__c(Name='First'),new LedgerItem__c(Name='Second')};
insert rows;
System.assertEquals(2,Limits.getDmlRows());
delete rows[0];
System.assertEquals(3,Limits.getDmlRows());
List<Database.DeleteResult> results=Database.delete(rows,false);
System.assertEquals(false,results[0].isSuccess());
System.assertEquals(true,results[1].isSuccess());
System.assertEquals(3,Limits.getDmlStatements());
System.assertEquals(5,Limits.getDmlRows());
System.assertEquals(0,[SELECT count() FROM LedgerItem__Share]);
`},
		{"hardDelete", `
List<LedgerItem__c> rows=new List<LedgerItem__c>{new LedgerItem__c(Name='First'),new LedgerItem__c(Name='Second')};
insert rows;
delete rows;
System.assertEquals(4,Limits.getDmlRows());
List<Database.EmptyRecycleBinResult> results=Database.emptyRecycleBin(rows);
System.assertEquals(true,results[0].isSuccess());
System.assertEquals(true,results[1].isSuccess());
System.assertEquals(3,Limits.getDmlStatements());
System.assertEquals(6,Limits.getDmlRows());
System.assertEquals(0,[SELECT count() FROM LedgerItem__Share]);
`},
		{"ordinaryCascadeChildren", `
LedgerItem__c parent=new LedgerItem__c(Name='Parent');
insert parent;
insert new List<LedgerDetail__c>{new LedgerDetail__c(Name='First',Parent__c=parent.Id),new LedgerDetail__c(Name='Second',Parent__c=parent.Id)};
System.assertEquals(3,Limits.getDmlRows());
delete parent;
System.assertEquals(3,Limits.getDmlStatements());
System.assertEquals(6,Limits.getDmlRows());
System.assertEquals(0,[SELECT count() FROM LedgerDetail__c]);
System.assertEquals(0,[SELECT count() FROM LedgerItem__Share]);
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := ownerShareDeleteLimitsOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func ownerShareDeleteLimitsOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["LedgerItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "LedgerItem__c", KeyPrefix: "a10", SharingModel: "Private",
			Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["LedgerItem__Share"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "LedgerItem__Share", KeyPrefix: "a11",
			Metadata: map[string]string{"kind": "generatedShare", "parentObject": "LedgerItem__c"},
			Fields: map[string]storage.Field{
				"ParentId":      {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"LedgerItem__c"}},
				"UserOrGroupId": {APIName: "UserOrGroupId", Type: storage.FieldReference, ReferenceTo: []string{"User", "Group"}},
				"RowCause":      {APIName: "RowCause", Type: storage.FieldPicklist},
				"AccessLevel":   {APIName: "AccessLevel", Type: storage.FieldPicklist},
			},
			Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"LedgerItem__c"}, CascadeDelete: true}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["LedgerDetail__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "LedgerDetail__c", KeyPrefix: "a12", SharingModel: "ControlledByParent",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"LedgerItem__c"}},
			},
			Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"LedgerItem__c"}, CascadeDelete: true}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}
