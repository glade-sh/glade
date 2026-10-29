package apextest

import (
	"path/filepath"
	"testing"
)

// Native rows (API 62.0 and 67.0, Developer scratch org; see
// internal/apextest/testdata/runtimegaps/flow_record_prior_reference/org.tsv):
// an after-save CreateAndUpdate flow whose decision compares $Record.Name and
// $Record.IsActive NotEqualTo the same field on $Record__Prior fires on insert,
// on a changed value and not on an update that leaves both fields unchanged,
// including a partial sObject that omits them.
func TestFlowDecisionComparesRecordFieldToPriorReference(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Prior_Reference.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>62.0</apiVersion>
  <actionCalls>
    <name>Push</name><label>Push</label><locationX>176</locationX><locationY>350</locationY>
    <actionName>PriorRefCounter</actionName><actionType>apex</actionType>
    <inputParameters><name>productId</name><value><elementReference>$Record.Id</elementReference></value></inputParameters>
    <nameSegment>PriorRefCounter</nameSegment>
  </actionCalls>
  <decisions>
    <name>Changed</name><label>Changed</label><locationX>176</locationX><locationY>242</locationY>
    <defaultConnectorLabel>No</defaultConnectorLabel>
    <rules>
      <name>Yes</name><conditionLogic>or</conditionLogic>
      <conditions><leftValueReference>$Record.Name</leftValueReference><operator>NotEqualTo</operator><rightValue><elementReference>$Record__Prior.Name</elementReference></rightValue></conditions>
      <conditions><leftValueReference>$Record.IsActive</leftValueReference><operator>NotEqualTo</operator><rightValue><elementReference>$Record__Prior.IsActive</elementReference></rightValue></conditions>
      <connector><targetReference>Push</targetReference></connector><label>Yes</label>
    </rules>
  </decisions>
  <environments>Default</environments>
  <label>Prior Reference</label>
  <processType>AutoLaunchedFlow</processType>
  <start>
    <locationX>50</locationX><locationY>0</locationY>
    <connector><targetReference>Changed</targetReference></connector>
    <object>Product2</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordAfterSave</triggerType>
  </start>
  <status>Active</status>
</Flow>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PriorRefCounter.cls"), `
public class PriorRefCounter {
  public static Integer calls = 0;
  public class Request { @InvocableVariable(required=true) public Id productId; }
  @InvocableMethod(label='Push')
  public static void push(List<Request> rs) { calls += rs.size(); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PriorRefTest.cls"), `
@isTest
private class PriorRefTest {
  @isTest static void comparesAgainstPriorRecord() {
    Product2 p = new Product2(Name = 'Widget', Description = 'd', IsActive = true);
    insert p;
    String r = 'insert=' + PriorRefCounter.calls;
    Integer b = PriorRefCounter.calls;
    update new Product2(Id = p.Id, ProductCode = 'x');
    r += '|partialUnrelated=' + (PriorRefCounter.calls - b); b = PriorRefCounter.calls;
    Product2 q = [SELECT Id, Name FROM Product2 WHERE Id = :p.Id];
    q.ProductCode = 'y'; update q;
    r += '|queriedUnrelated=' + (PriorRefCounter.calls - b); b = PriorRefCounter.calls;
    update new Product2(Id = p.Id, Name = 'Widget');
    r += '|sameName=' + (PriorRefCounter.calls - b); b = PriorRefCounter.calls;
    update new Product2(Id = p.Id, Name = 'Other');
    r += '|newName=' + (PriorRefCounter.calls - b); b = PriorRefCounter.calls;
    update new Product2(Id = p.Id, IsActive = false);
    r += '|activeFlip=' + (PriorRefCounter.calls - b);
    System.assertEquals('insert=1|partialUnrelated=0|queriedUnrelated=0|sameName=0|newName=1|activeFlip=1', r);
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem = %q", got, firstRunProblem(run))
	}
}
