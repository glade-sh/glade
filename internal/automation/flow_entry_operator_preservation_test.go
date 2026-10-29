package automation

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// Uncaptured entry operators retain the base loader's criteria projection and
// acquire no rejection. These are preservation checks, not native parity
// assertions. The "1 OR 2" start logic is evaluated natively (the flow logic
// family's captured rows), so it no longer produces the unsupported-logic
// warning; the projection keeps each filter's own operator under two
// alternative groups.
func TestFlowEntryUncapturedOperatorsPreserveProjection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operator := range []string{"IsChanged", "Includes", "WasSet", "NotIn"} {
			t.Run(api+"/"+operator, func(t *testing.T) {
				root := t.TempDir()
				p := projectWithAccountTier(t, root, true)
				path := filepath.Join(root, "force-app/main/default/flows/Entry.flow-meta.xml")
				writeWorkflowTestFile(t, path, fmt.Sprintf(`<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status><start><object>Account</object><triggerType>RecordAfterSave</triggerType><recordTriggerType>CreateAndUpdate</recordTriggerType><filterLogic>1 OR 2</filterLogic><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>seed</stringValue></value></filters><filters><field>Tier__c</field><operator>%s</operator><value><stringValue>Gold</stringValue></value></filters></start><assignments><name>Mark</name><assignmentItems><assignToReference>$Record.Tier__c</assignToReference><operator>Assign</operator><value><stringValue>Gold</stringValue></value></assignmentItems></assignments></Flow>`, api, operator))
				p.FlowFiles = []string{path}
				index, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, diagnostic := range index.Diagnostics {
					if diagnostic.Code == "GLADEAUTO004" || diagnostic.Code == "GLADEAUTO002" {
						t.Fatalf("uncaptured entry operator acquired a diagnostic: %+v", diagnostic)
					}
				}
				if len(index.Flows) != 1 || len(index.Flows[0].Rules) != 1 {
					t.Fatalf("base projection lost: %+v", index)
				}
				rule := index.Flows[0].Rules[0]
				if len(rule.Criteria) != 2 || rule.Criteria[1].Operation != flowOperator(operator) || rule.Formula != "" {
					t.Fatalf("base entry criteria changed: %+v", rule)
				}
				if !reflect.DeepEqual(rule.CriteriaGroups, [][]int{{0}, {1}}) {
					t.Fatalf("OR start logic groups changed: %+v", rule.CriteriaGroups)
				}
			})
		}
	}
}
