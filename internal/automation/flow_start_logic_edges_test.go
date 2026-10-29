package automation

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestFlowStartLogicUnsupportedEdges(t *testing.T) {
	for _, logic := range []string{
		"NOT 1", "1 XOR 2", "(1 OR 2", "1 OR 2)", "1 OR ()", "0 OR 2", "1 OR 3",
		"18446744073709551617 OR 2", "((1 OR 2) AND (NOT 1))", " \t( 1 XOR\n 2 ) \r",
		"(1 OR -2)", "(-1 OR 2)", "NOT (1 OR -2)", "(1 OR +3)", "(1 OR +2)", "(", ")", "()",
	} {
		t.Run(fmt.Sprintf("%q", logic), func(t *testing.T) {
			idx, path := loadStartLogicEdge(t, logic)
			if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO002" || idx.Diagnostics[0].File != path || !strings.Contains(idx.Diagnostics[0].Message, "start filter logic") {
				t.Fatalf("diagnostics=%#v", idx.Diagnostics)
			}
			if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
				t.Fatalf("flows=%#v", idx.Flows)
			}
			rule := idx.Flows[0].Rules[0]
			if len(rule.Criteria) != 2 || len(rule.CriteriaGroups) != 0 || rule.Formula != "" {
				t.Fatalf("fallback rule=%#v", rule)
			}
		})
	}
}

func TestFlowStartLogicPreservesFormulaFallback(t *testing.T) {
	for _, expression := range []string{`(Name = "Alpha")`, "ISBLANK(Name)", "NOT(ISBLANK(Name))", "ISBLANK(1)", "ISBLANK(-1)"} {
		if got := flowCriteriaFormula(expression, nil); got != expression {
			t.Errorf("formula %q became %q", expression, got)
		}
	}
	if got := flowCriteriaFormula("Should_Run", map[string]string{"should_run": "ISBLANK(Name)"}); got != "ISBLANK(Name)" {
		t.Errorf("named formula became %q", got)
	}
}

func TestFlowStartLogicNestedWhitespace(t *testing.T) {
	for _, logic := range []string{"((1 OR 2) AND (1))", " \t(( 1 oR\n2 ) aNd\r ( 1 )) \t"} {
		t.Run(fmt.Sprintf("%q", logic), func(t *testing.T) {
			idx, _ := loadStartLogicEdge(t, logic)
			if len(idx.Diagnostics) != 0 || len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
				t.Fatalf("index=%#v", idx)
			}
			if got := idx.Flows[0].Rules[0].CriteriaGroups; !reflect.DeepEqual(got, [][]int{{0, 0}, {1, 0}}) {
				t.Fatalf("groups=%v", got)
			}
		})
	}
}

func loadStartLogicEdge(t *testing.T, logic string) (Index, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "force-app/main/default/flows/LogicEdge.flow-meta.xml")
	writeWorkflowTestFile(t, path, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><processType>AutoLaunchedFlow</processType><status>Active</status><start><object>Account</object><triggerType>RecordBeforeSave</triggerType><recordTriggerType>Create</recordTriggerType><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Alpha</stringValue></value></filters><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Beta</stringValue></value></filters><filterLogic>`+logic+`</filterLogic><connector><targetReference>Mark</targetReference></connector></start><assignments><name>Mark</name><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>matched</stringValue></value></assignmentItems></assignments></Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	return idx, path
}
