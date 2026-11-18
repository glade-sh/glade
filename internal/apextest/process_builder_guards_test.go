package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pbGuardMeta = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

func pbGuardObject(t *testing.T, base, name string) {
	t.Helper()
	dir := filepath.Join(base, "objects", name, "fields")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pbGuardWrite(t, filepath.Join(base, "objects", name, name+".object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>L</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Ls</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	for _, field := range []string{"Stage__c", "Note__c", "Old__c"} {
		pbGuardWrite(t, filepath.Join(dir, field+".field-meta.xml"), fmt.Sprintf(`<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>%s</fullName><label>F</label><length>80</length><type>Text</type></CustomField>`, field))
	}
}

func pbGuardWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(pbGuardMeta+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pbGuardProcess(t *testing.T, base, flow, object, conditions, formulas, field string) {
	t.Helper()
	pbGuardWrite(t, filepath.Join(base, "flows", flow+".flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>49.0</apiVersion><decisions><name>myDecision</name><label>d</label><defaultConnectorLabel>default</defaultConnectorLabel><rules><name>myRule_1</name><conditionLogic>and</conditionLogic>`+conditions+`<connector><targetReference>myRule_1_A1</targetReference></connector><label>r</label></rules></decisions>`+formulas+`<label>`+flow+`</label><processMetadataValues><name>ObjectType</name><value><stringValue>`+object+`</stringValue></value></processMetadataValues><processMetadataValues><name>ObjectVariable</name><value><elementReference>myVariable_current</elementReference></value></processMetadataValues><processMetadataValues><name>OldObjectVariable</name><value><elementReference>myVariable_old</elementReference></value></processMetadataValues><processMetadataValues><name>TriggerType</name><value><stringValue>onAllChanges</stringValue></value></processMetadataValues><processType>Workflow</processType><recordUpdates><name>myRule_1_A1</name><label>u</label><filterLogic>and</filterLogic><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>`+field+`</field><value><stringValue>HIT</stringValue></value></inputAssignments><object>`+object+`</object></recordUpdates><startElementReference>myDecision</startElementReference><status>Active</status><variables><name>myVariable_current</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>true</isOutput><objectType>`+object+`</objectType></variables><variables><name>myVariable_old</name><dataType>SObject</dataType><isCollection>false</isCollection><isInput>true</isInput><isOutput>false</isOutput><objectType>`+object+`</objectType></variables></Flow>`)
}

const pbGuardStageIsA = `<conditions><leftValueReference>myVariable_current.Stage__c</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>a</stringValue></rightValue></conditions>`

func pbGuardAfterSave(t *testing.T, base, flow, object, trigger, body, target string) {
	t.Helper()
	pbGuardWrite(t, filepath.Join(base, "flows", flow+".flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>`+flow+`</label><processType>AutoLaunchedFlow</processType>`+body+`<start><connector><targetReference>`+target+`</targetReference></connector><object>`+object+`</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>`+trigger+`</triggerType></start><status>Active</status></Flow>`)
}

// Guards around the process_builder_shapes path. Each replays a source that ran
// or saved at the base commit and failed the save once processes ran in bulk.
func TestProcessBuilderGuards(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app", "main", "default")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories": [{"path": "force-app", "default": true}], "sourceApiVersion": "67.0", "namespace": ""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	objects := []string{"Oak_Ids__c", "Oak_Kid__c", "Oak_Func__c", "Oak_Two__c", "Oak_Prior__c", "Oak_Before__c", "Oak_Bulk__c"}
	permissions := ""
	for _, object := range objects {
		pbGuardObject(t, base, object)
		permissions += `<objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>true</modifyAllRecords><object>` + object + `</object><viewAllRecords>true</viewAllRecords></objectPermissions>`
		for _, field := range []string{"Stage__c", "Note__c", "Old__c"} {
			permissions = `<fieldPermissions><editable>true</editable><field>` + object + "." + field + `</field><readable>true</readable></fieldPermissions>` + permissions
		}
	}
	pbGuardWrite(t, filepath.Join(base, "permissionsets", "Oak.permissionset-meta.xml"), `<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>Oak</label>`+permissions+`</PermissionSet>`)
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("test:\n  permissionSets: [Oak]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An after-save flow on a process object creates a record: the next save
	// must not issue its id again.
	pbGuardProcess(t, base, "Oak_Ids_P", "Oak_Ids__c", pbGuardStageIsA, "", "Note__c")
	pbGuardAfterSave(t, base, "Oak_Ids_F", "Oak_Ids__c", "RecordAfterSave", `<recordCreates><name>C</name><label>C</label><inputAssignments><field>Name</field><value><stringValue>kid</stringValue></value></inputAssignments><object>Oak_Kid__c</object></recordCreates>`, "C")
	// A criterion the formula evaluator cannot read leaves the process unrun.
	pbGuardProcess(t, base, "Oak_Func_P", "Oak_Func__c", `<conditions><leftValueReference>formula_myRule_1</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions>`, `<formulas><name>formula_myRule_1</name><dataType>Boolean</dataType><expression>GETSESSIONID() = "x"</expression></formulas>`, "Note__c")
	// No row pins the order between two processes of one object.
	pbGuardProcess(t, base, "Oak_Two_P", "Oak_Two__c", pbGuardStageIsA, "", "Note__c")
	pbGuardProcess(t, base, "Oak_Two_Q", "Oak_Two__c", pbGuardStageIsA, "", "Old__c")
	// Ordinary flows have no prior-value row: the save stays as it was.
	priorUpdate := `<recordUpdates><name>U</name><label>U</label><inputAssignments><field>Old__c</field><value><elementReference>$Record__Prior.Stage__c</elementReference></value></inputAssignments><inputReference>$Record</inputReference></recordUpdates>`
	pbGuardAfterSave(t, base, "Oak_Prior_F", "Oak_Prior__c", "RecordAfterSave", priorUpdate, "U")
	pbGuardAfterSave(t, base, "Oak_Before_F", "Oak_Before__c", "RecordBeforeSave", priorUpdate, "U")
	// A bulk update on a process object has no batched after-save action row.
	pbGuardProcess(t, base, "Oak_Bulk_P", "Oak_Bulk__c", pbGuardStageIsA, "", "Note__c")
	pbGuardAfterSave(t, base, "Oak_Bulk_F", "Oak_Bulk__c", "RecordAfterSave", `<actionCalls><name>M</name><label>M</label><actionName>OakMark</actionName><actionType>apex</actionType><flowTransactionModel>CurrentTransaction</flowTransactionModel><inputParameters><name>rows</name><value><elementReference>$Record</elementReference></value></inputParameters></actionCalls>`, "M")
	class := func(name, body string) {
		if err := os.MkdirAll(filepath.Join(base, "classes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "classes", name+".cls"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		pbGuardWrite(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	class("OakMark", `public class OakMark { public static List<Integer> sizes = new List<Integer>(); @InvocableMethod public static void mark(List<Oak_Bulk__c> rows) { sizes.add(rows.size()); } }`)
	class("OakGuardTest", `@IsTest public class OakGuardTest {
    @IsTest static void ids() {
        insert new Oak_Ids__c(Name = '1', Stage__c = 'a');
        insert new Oak_Kid__c(Name = 'direct');
        insert new Oak_Ids__c(Name = '2', Stage__c = 'a');
        insert new Oak_Kid__c(Name = 'direct2');
        System.assertEquals(4, [SELECT COUNT() FROM Oak_Kid__c]);
    }
    @IsTest static void unreadableCriterion() {
        insert new Oak_Func__c(Name = 'f', Stage__c = 'a');
        System.assertEquals(null, [SELECT Note__c FROM Oak_Func__c].Note__c);
    }
    @IsTest static void twoProcesses() {
        insert new Oak_Two__c(Name = 't', Stage__c = 'a');
        Oak_Two__c row = [SELECT Note__c, Old__c FROM Oak_Two__c];
        System.assertEquals(null, row.Note__c);
        System.assertEquals(null, row.Old__c);
    }
    @IsTest static void ordinaryPrior() {
        Oak_Prior__c after = new Oak_Prior__c(Name = 'p', Stage__c = 'a');
        insert after;
        after.Stage__c = 'b';
        update after;
        Oak_Before__c before = new Oak_Before__c(Name = 'p', Stage__c = 'a');
        insert before;
        before.Stage__c = 'b';
        update before;
    }
    @IsTest static void bulkUpdateAction() {
        List<Oak_Bulk__c> rows = new List<Oak_Bulk__c>{ new Oak_Bulk__c(Name = 'b1'), new Oak_Bulk__c(Name = 'b2') };
        insert rows;
        System.assertEquals(new List<Integer>{ 2 }, OakMark.sizes);
        OakMark.sizes.clear();
        update rows;
        System.assertEquals(new List<Integer>{ 2 }, OakMark.sizes);
    }
}`)
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"OakGuardTest"}, NoDiskCache: true, Parallelism: 1})
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			seen++
			if c.Problem != nil {
				t.Errorf("%s: %s", c.MethodName, strings.TrimSpace(c.Problem.Message))
			}
		}
	}
	if seen != 5 {
		t.Fatalf("ran %d cases, want 5: %+v", seen, run.Summary())
	}
}
