package automation

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native flow-target-reject (API 62.0 and 67.0): an Assignment that merges a
// flow resource into a Number, Boolean or Date variable through a string value
// fails the deploy with the message below. A String target compiles.
func TestFlowMergeIntoNonStringTargetIsRejected(t *testing.T) {
	flow := func(target, targetType string) string {
		return `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>67.0</apiVersion>
<assignments><name>SetOut</name><label>SetOut</label><locationX>1</locationX><locationY>1</locationY>
<assignmentItems><assignToReference>` + target + `</assignToReference><operator>Assign</operator><value><stringValue>{!BoolVar}</stringValue></value></assignmentItems></assignments>
<processType>AutoLaunchedFlow</processType><label>F</label>
<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>SetOut</targetReference></connector></start>
<status>Active</status>
<variables><name>BoolVar</name><dataType>Boolean</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><booleanValue>true</booleanValue></value></variables>
<variables><name>` + target + `</name><dataType>` + targetType + `</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>true</isOutput></variables>
</Flow>`
	}
	for _, test := range []struct {
		target, targetType string
		rejected           bool
	}{{"BoolOut", "Boolean", true}, {"StrOut", "String", false}} {
		dir := t.TempDir()
		path := filepath.Join(dir, "F.flow-meta.xml")
		if err := os.WriteFile(path, []byte(flow(test.target, test.targetType)), 0o600); err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := loadFlow(path)
		if err != nil {
			t.Fatal(err)
		}
		var message string
		for _, d := range diagnostics {
			if strings.Contains(d.Message, "isn’t a valid value") {
				message = d.Message
			}
		}
		want := "field integrity exception: unknown (In an Assignment element, “{!BoolVar}” isn’t a valid value when the “BoolOut” variable is set with the Equals operator.)"
		if test.rejected && message != want {
			t.Fatalf("diagnostics = %#v", diagnostics)
		}
		if !test.rejected && message != "" {
			t.Fatalf("a String target was rejected: %q", message)
		}
	}
}

// Resources the capture did not cover (Currency, DateTime, a Number without a
// declared scale) keep their earlier handling: the merge text stays as written.
func TestFlowMergeLeavesUncapturedResourceTypes(t *testing.T) {
	source := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><processType>AutoLaunchedFlow</processType>
<assignments><name>A</name><assignmentItems><assignToReference>Out</assignToReference><operator>Assign</operator><value><stringValue>c={!Cur}</stringValue></value></assignmentItems>
<assignmentItems><assignToReference>Out</assignToReference><operator>Assign</operator><value><stringValue>{!Dt}</stringValue></value></assignmentItems>
<assignmentItems><assignToReference>Out</assignToReference><operator>Assign</operator><value><stringValue>n={!NoScale}</stringValue></value></assignmentItems></assignments>
<variables><name>Out</name><dataType>String</dataType></variables>
<variables><name>Cur</name><dataType>Currency</dataType><scale>2</scale></variables>
<variables><name>Dt</name><dataType>DateTime</dataType></variables>
<variables><name>NoScale</name><dataType>Number</dataType></variables></Flow>`
	var raw flowXML
	if err := xml.Unmarshal([]byte(source), &raw); err != nil {
		t.Fatal(err)
	}
	flowMergeFormulaStringAssignments(&raw)
	for i, want := range []string{"c={!Cur}", "{!Dt}", "n={!NoScale}"} {
		item := raw.Assignments[0].Items[i]
		if item.Value.StringValue != want || item.Value.ElementReference != "" {
			t.Fatalf("item %d rewritten: %+v", i, item.Value)
		}
	}
}
