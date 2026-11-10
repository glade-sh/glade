package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The asg rows capture one start assignment and a private DateTime resource.
func TestProcessBuilderWaitStartBoundaries(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_shapes", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "force-app/main/default/flows/Fern_Asg_Process.flow-meta.xml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := string(b)
	replace := func(old, new string) string {
		t.Helper()
		if !strings.Contains(original, old) {
			t.Fatalf("fixture missing %q", old)
		}
		return strings.Replace(original, old, new, 1)
	}
	changeVariable := func(old, new string) string {
		t.Helper()
		start := strings.Index(original, "<variables><name>myVariable_waitStartTimeVariable</name>")
		if start < 0 {
			t.Fatal("start variable missing")
		}
		end := strings.Index(original[start:], "</variables>")
		if end < 0 {
			t.Fatal("start variable is not closed")
		}
		end += start + len("</variables>")
		segment := original[start:end]
		if !strings.Contains(segment, old) {
			t.Fatalf("variable missing %q", old)
		}
		return original[:start] + strings.Replace(segment, old, new, 1) + original[end:]
	}
	renamed := strings.NewReplacer("myVariable_waitStartTimeAssignment", "renamedStart", "myVariable_waitStartTimeVariable", "renamedClock").Replace(original)
	cases := []struct {
		name, source string
		admitted     bool
	}{
		{"captured", original, true},
		{"renamed resources", renamed, true},
		{"invalid resource identifier", strings.ReplaceAll(original, "myVariable_waitStartTimeVariable", "invalid-name"), false},
		{"wrong target", replace("<assignToReference>myVariable_waitStartTimeVariable</assignToReference>", "<assignToReference>myVariable_current.Stage__c</assignToReference>"), false},
		{"wrong operator", replace("<operator>Assign</operator>", "<operator>Add</operator>"), false},
		{"wrong assignment value", replace("<elementReference>$Flow.CurrentDateTime</elementReference>", "<elementReference>myVariable_current.Id</elementReference>"), false},
		{"assignment value with extra literal", replace("<elementReference>$Flow.CurrentDateTime</elementReference>", "<elementReference>$Flow.CurrentDateTime</elementReference><stringValue>extra</stringValue>"), false},
		{"additional assignment item", replace("</assignmentItems>", "</assignmentItems><assignmentItems><assignToReference>myVariable_current.Stage__c</assignToReference><operator>Assign</operator><value><stringValue>x</stringValue></value></assignmentItems>"), false},
		{"fault connector", replace("</assignments>", "<faultConnector><targetReference>myDecision</targetReference></faultConnector></assignments>"), false},
		{"wrong connector", replace("<targetReference>myDecision</targetReference>", "<targetReference>missingDecision</targetReference>"), false},
		{"start bypasses assignment", replace("<startElementReference>myVariable_waitStartTimeAssignment</startElementReference>", "<startElementReference>myDecision</startElementReference>"), false},
		{"variable type", changeVariable("<dataType>DateTime</dataType>", "<dataType>String</dataType>"), false},
		{"variable input", changeVariable("<isInput>false</isInput>", "<isInput>true</isInput>"), false},
		{"variable output", changeVariable("<isOutput>false</isOutput>", "<isOutput>true</isOutput>"), false},
		{"variable collection", changeVariable("<isCollection>false</isCollection>", "<isCollection>true</isCollection>"), false},
		{"variable default", changeVariable("<elementReference>$Flow.CurrentDateTime</elementReference>", "<elementReference>$Flow.CurrentDate</elementReference>"), false},
		{"extra variable property", changeVariable("</variables>", "<scale>2</scale></variables>"), false},
		{"variable read", replace("</Flow>", "<formulas><name>ClockRead</name><dataType>DateTime</dataType><expression>{!myVariable_waitStartTimeVariable}</expression></formulas></Flow>"), false},
		{"variable used by update", replace("<stringValue>A</stringValue>", "<elementReference>myVariable_waitStartTimeVariable</elementReference>"), false},
		{"duplicate variable", replace("</Flow>", "<variables><name>myVariable_waitStartTimeVariable</name><dataType>DateTime</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><elementReference>$Flow.CurrentDateTime</elementReference></value></variables></Flow>"), false},
		{"unknown assignment property", replace("<assignmentItems>", "<unexpected>true</unexpected><assignmentItems>"), false},
	}
	defs := flowProjectRelationshipDefinitions(p)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, skip := normalizeProcessBuilderFlow([]byte(tc.source), defs)
			if got := skip == ""; got != tc.admitted {
				t.Fatalf("admitted=%t, want %t; skip=%q", got, tc.admitted, skip)
			}
		})
	}
}
