package apextest

import (
	"path/filepath"
	"testing"
)

// Native self-lookup results: flow_relationship_id_target_boundary/org.tsv:2-9.
func TestStandardFieldEvidenceFlowExistingBoundaryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/flows/OwnedCollection.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>`+api+`</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>
<start><object>Contact</object><triggerType>RecordBeforeSave</triggerType><recordTriggerType>Create</recordTriggerType><connector><targetReference>FindRows</targetReference></connector></start>
<recordLookups><name>FindRows</name><object>Account</object><getFirstRecordOnly>false</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically><connector><targetReference>CopyParent</targetReference></connector></recordLookups>
<recordUpdates><name>CopyParent</name><inputReference>$Record</inputReference><inputAssignments><field>AccountId</field><value><elementReference>$Record.Account.Id</elementReference></value></inputAssignments></recordUpdates>
</Flow>`)
			path := filepath.Join(root, "force-app/main/default/classes/OwnedFlowIsolationTest.cls")
			writeFile(t, path, `@IsTest private class OwnedFlowIsolationTest {
@IsTest static void sample(){
Account parent=new Account(Name='Owned parent'); insert parent;
Contact child=new Contact(LastName='Owned child',AccountId=parent.Id); insert child;
Contact saved=[SELECT AccountId FROM Contact WHERE Id=:child.Id];
System.assertEquals(parent.Id,saved.AccountId);
Contact empty=new Contact(LastName='Empty'); insert empty;
Contact savedEmpty=[SELECT AccountId FROM Contact WHERE Id=:empty.Id];
System.assertEquals(null,savedEmpty.AccountId);
}}`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Failed != 0 {
				t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
