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

// The native deploy failures of flow-target-reject, two repeats identical
// (parallel/batch2-20261117/flow-target-reject r1 and r2, raw/deploy-fails-1.txt;
// copied to testdata/flow_target_reject/deploy-fails.tsv, id then message). Each
// flow is an Assignment of a string value that is exactly one merge into a
// variable of the same type: a Number, Boolean or Date variable, and a Number
// formula, with a Number target of scale 2.
func TestFlowTargetRejectRowsAreRejected(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "flow_target_reject", "deploy-fails.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	shapes := map[string]struct{ source, target string }{
		"Ft_bool_target":     {"BoolVar", "BoolOut"},
		"Ft_date_target":     {"DateVar", "DateOut"},
		"Ft_num_target":      {"NumVar", "NumOut"},
		"Ft_num_target_text": {"NumF", "NumOut"},
	}
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		id, message, _ := strings.Cut(line, "\t")
		shape, ok := shapes[id]
		if !ok {
			t.Fatalf("row %s has no shape", id)
		}
		seen++
		if got := mergeTargetDiagnostic(t, targetFlow(shape.source, shape.target)); got != message {
			t.Fatalf("%s: got %q, want native %q", id, got, message)
		}
	}
	if seen != len(shapes) {
		t.Fatalf("rows seen %d, want %d", seen, len(shapes))
	}
}

// Shapes that no native row covers keep the text they had before typed merges
// (d5ae42fedb^): no deploy rejection. The string value stays as written.
func TestFlowTargetRejectUncoveredShapesKeepEarlierText(t *testing.T) {
	for _, test := range []struct{ name, source, target, text string }{
		{"Boolean formula into Boolean", "BoolF", "BoolOut", "{!BoolF}"},
		{"Date formula into Date", "DateF", "DateOut", "{!DateF}"},
		{"String variable into Number", "StrVar", "NumOut", "{!StrVar}"},
		{"String formula into Boolean", "StrF", "BoolOut", "{!StrF}"},
		{"Number variable into Date", "NumVar", "DateOut", "{!NumVar}"},
		{"Date variable into Number", "DateVar", "NumOut", "{!DateVar}"},
		{"Boolean variable into Date", "BoolVar", "DateOut", "{!BoolVar}"},
		{"Number variable text into Number", "NumVar", "NumOut", "n={!NumVar}"},
		{"Date variable text into Date", "DateVar", "DateOut", "d={!DateVar}"},
	} {
		flow := strings.Replace(targetFlow(test.source, test.target), "{!"+test.source+"}", test.text, 1)
		if got := mergeTargetDiagnostic(t, flow); got != "" {
			t.Fatalf("%s: rejected without a native row: %q", test.name, got)
		}
	}
}

func targetFlow(source, target string) string {
	targetType := map[string]string{"BoolOut": "Boolean", "DateOut": "Date", "NumOut": "Number"}[target]
	scale := ""
	if targetType == "Number" {
		scale = "<scale>2</scale>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
<apiVersion>67.0</apiVersion>
<assignments><name>SetOut</name><label>SetOut</label><locationX>1</locationX><locationY>1</locationY>
<assignmentItems><assignToReference>` + target + `</assignToReference><operator>Assign</operator><value><stringValue>{!` + source + `}</stringValue></value></assignmentItems></assignments>
<formulas><name>NumF</name><dataType>Number</dataType><scale>2</scale><expression>3.5 * 2</expression></formulas>
<formulas><name>BoolF</name><dataType>Boolean</dataType><expression>1 = 1</expression></formulas>
<formulas><name>DateF</name><dataType>Date</dataType><expression>DATE(2026, 1, 2)</expression></formulas>
<formulas><name>StrF</name><dataType>String</dataType><expression>"ab" &amp; "cd"</expression></formulas>
<processType>AutoLaunchedFlow</processType><label>F</label>
<start><locationX>0</locationX><locationY>0</locationY><connector><targetReference>SetOut</targetReference></connector></start>
<status>Active</status>
<variables><name>NumVar</name><dataType>Number</dataType><scale>2</scale><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><numberValue>2.5</numberValue></value></variables>
<variables><name>BoolVar</name><dataType>Boolean</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><booleanValue>true</booleanValue></value></variables>
<variables><name>DateVar</name><dataType>Date</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><dateValue>2026-01-02</dateValue></value></variables>
<variables><name>StrVar</name><dataType>String</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>false</isOutput><value><stringValue>abc</stringValue></value></variables>
<variables><name>` + target + `</name><dataType>` + targetType + `</dataType><isCollection>false</isCollection><isInput>false</isInput><isOutput>true</isOutput>` + scale + `</variables>
</Flow>`
}

func mergeTargetDiagnostic(t *testing.T, flow string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "F.flow-meta.xml")
	if err := os.WriteFile(path, []byte(flow), 0o600); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := loadFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diagnostics {
		if strings.Contains(d.Message, "isn’t a valid value") {
			return d.Message
		}
	}
	return ""
}
