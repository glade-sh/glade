package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
)

// TestRunKeepsAutomationWhenOneFlowFileIsUnparsable checks that a malformed
// flow file is reported and skipped while the valid flow and the trigger on
// the same object still run.
func TestRunKeepsAutomationWhenOneFlowFileIsUnparsable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Flow_Status__c.field-meta.xml"), `
<CustomField>
  <fullName>Flow_Status__c</fullName>
  <label>Flow Status</label>
  <type>Text</type>
  <length>40</length>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Account_Status.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Account</object>
    <triggerType>RecordBeforeSave</triggerType>
    <connector><targetReference>Set_Status</targetReference></connector>
  </start>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Flow_Status__c</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>flowed</stringValue></value>
    </assignmentItems>
  </assignments>
</Flow>`)
	brokenPath := filepath.Join(root, "force-app/main/default/flows/Broken_Flow.flow-meta.xml")
	writeFile(t, brokenPath, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Account</object>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/AccountNameTrigger.trigger"), `
trigger AccountNameTrigger on Account (before insert) {
  for (Account a : Trigger.new) {
    a.Name = a.Name + '!';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/FlowIsolationTest.cls"), `
@isTest
private class FlowIsolationTest {
  @isTest static void validFlowAndTriggerStillRun() {
    Account a = new Account(Name = 'Acme');
    insert a;
    Account row = [SELECT Name, Flow_Status__c FROM Account WHERE Id = :a.Id];
    System.assertEquals('Acme!', row.Name);
    System.assertEquals('flowed', row.Flow_Status__c);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem = %q", got, firstRunProblem(run))
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO003" || !sameFlowTestPath(idx.Diagnostics[0].File, brokenPath) ||
		!strings.Contains(idx.Diagnostics[0].Message, "Broken_Flow") {
		t.Fatalf("diagnostics = %#v", idx.Diagnostics)
	}

	// Control: with only valid automation the load has no warnings and the
	// same test passes.
	if err := os.Remove(brokenPath); err != nil {
		t.Fatal(err)
	}
	control := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := control.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("control summary = %#v problem = %q", got, firstRunProblem(control))
	}
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err = automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("control diagnostics = %#v", idx.Diagnostics)
	}
}

func sameFlowTestPath(a, b string) bool {
	left, errLeft := filepath.EvalSymlinks(a)
	right, errRight := filepath.EvalSymlinks(b)
	if errLeft != nil || errRight != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return left == right
}
