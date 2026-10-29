package apextest

import (
	"fmt"
	"strings"
	"testing"
)

// A filtered update of the triggering record, reached through an OR start,
// must not overwrite values supplied by the caller on an unrelated update.
// This is a preservation regression for the previously accepted runtime; it
// does not claim a new native observation for filtered Update Records.
func TestFlowFilteredUpdatePreservesCallerValues(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []string{"Description", "Phone", "Fax"} {
			t.Run(api+"/"+field, func(t *testing.T) {
				flow := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><label>Filtered</label><processType>AutoLaunchedFlow</processType><status>Active</status>
<decisions><name>Choose</name><label>Choose</label><defaultConnectorLabel>Other</defaultConnectorLabel><rules><name>Hot</name><label>Hot</label><conditionLogic>and</conditionLogic><conditions><leftValueReference>$Record.Rating</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Hot</stringValue></rightValue></conditions><connector><targetReference>Write</targetReference></connector></rules></decisions>
<recordUpdates><name>Write</name><label>Write</label><filterLogic>and</filterLogic><filters><field>FIELD</field><operator>IsNull</operator><value><booleanValue>true</booleanValue></value></filters><inputAssignments><field>FIELD</field><value><stringValue>flow-value</stringValue></value></inputAssignments><inputReference>$Record</inputReference></recordUpdates>
<start><connector><targetReference>Choose</targetReference></connector><filterLogic>or</filterLogic><filters><field>Rating</field><operator>IsChanged</operator><value><booleanValue>true</booleanValue></value></filters><filters><field>Rating</field><operator>EqualTo</operator><value><stringValue>Hot</stringValue></value></filters><object>Account</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordAfterSave</triggerType></start></Flow>`
				flow = strings.ReplaceAll(flow, "FIELD", field)
				classes := map[string]string{"FilterPreservationTest": fmt.Sprintf(`@IsTest private class FilterPreservationTest {
@IsTest static void observe() {
    for (String value : new List<String>{'first', 'second', 'third'}) {
        Account a = new Account(Name='Example', Rating='Hot'); insert a;
        a.%s=value; update a;
        Account result = [SELECT %s FROM Account WHERE Id=:a.Id];
        System.assertEquals(value, result.%s);
    }
}}`, field, field, field)}
				files := map[string]string{"force-app/main/default/flows/Filtered.flow-meta.xml": flow}
				index := flowRefsIndex(t, api, classes, files)
				result := Run(index, Options{SelectedClasses: []string{"FilterPreservationTest"}, NoDiskCache: true, Parallelism: 1})
				if result.Summary().Total != 1 || result.Summary().Failed != 0 {
					t.Fatalf("regression: %s", firstRunProblem(result))
				}
			})
		}
	}
}
