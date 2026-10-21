package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Salesforce documents Update Records using the ID and field values stored in
// a record variable by earlier Assignment elements. These owned project cases
// exercise the metadata loader and the shared DML path, without asserting a
// native API/context observation.
// https://help.salesforce.com/s/articleView?id=platform.flow_ref_elements_data_update.htm&language=en_US
func TestRunFlowUpdatesAssignedSingleRecordVariable(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, collection := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/collection=%t", api, collection), func(t *testing.T) {
				root := t.TempDir()
				target := "pending"
				connector := "PersistPending"
				collect := ""
				variables := `<variables><name>pending</name><dataType>SObject</dataType><objectType>Account</objectType><isCollection>false</isCollection></variables>`
				if collection {
					target, connector = "pendingRows", "CollectPending"
					collect = `<assignments><name>CollectPending</name><assignmentItems><assignToReference>pendingRows</assignToReference><operator>Add</operator><value><elementReference>pending</elementReference></value></assignmentItems><connector><targetReference>PersistPending</targetReference></connector></assignments>`
					variables += `<variables><name>pendingRows</name><dataType>SObject</dataType><objectType>Account</objectType><isCollection>true</isCollection></variables>`
				}
				writeFile(t, filepath.Join(root, "flows", "OwnedVariableUpdate.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>`+api+`</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<assignments><name>PreparePending</name>
<assignmentItems><assignToReference>pending.Id</assignToReference><operator>Assign</operator><value><elementReference>$Record.Id</elementReference></value></assignmentItems>
<assignmentItems><assignToReference>pending.Description</assignToReference><operator>Assign</operator><value><elementReference>$Record.Name</elementReference></value></assignmentItems>
<connector><targetReference>`+connector+`</targetReference></connector></assignments>
`+collect+`
<recordUpdates><name>PersistPending</name><inputReference>`+target+`</inputReference></recordUpdates>
`+variables+`
<start><connector><targetReference>PreparePending</targetReference></connector><object>Account</object><triggerType>RecordAfterSave</triggerType><recordTriggerType>Create</recordTriggerType>
<filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Sprout</stringValue></value></filters></start>
</Flow>`)
				writeFile(t, filepath.Join(root, "OwnedVariableUpdateTest.cls"), `@IsTest private class OwnedVariableUpdateTest {
@IsTest static void persistAssignedFields() {
Account row=new Account(Name='Sprout',Description='before'); insert row;
Account saved=[SELECT Name,Description FROM Account WHERE Id=:row.Id];
System.assertEquals('Sprout',saved.Name); System.assertEquals('Sprout',saved.Description);
System.assertEquals(1,[SELECT COUNT() FROM Account]);
}
@IsTest static void entryFilterControl() {
Account row=new Account(Name='Other',Description='unchanged'); insert row;
Account saved=[SELECT Name,Description FROM Account WHERE Id=:row.Id];
System.assertEquals('Other',saved.Name); System.assertEquals('unchanged',saved.Description);
}
}`)
				writeFile(t, filepath.Join(root, "OwnedVariableUpdateTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 {
					t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
				}
			})
		}
	}
}
