package automation

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured process formulas are in process_builder_shapes/org.tsv (upd,
// fan, pri, cgl), plus process_builder_wait/org.tsv. Uncaptured syntax must
// leave the whole process unrun rather than use the evaluator's larger surface.
func TestProcessBuilderFormulaEvidenceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		expression string
		admitted   bool
	}{
		{`true`, true},
		{`BLANKVALUE({!myVariable_current.Amount__c},0)+1`, true},
		{`IF({!myVariable_current.Stage__c}="bad",99,1)`, true},
		{`UPPER({!myVariable_current.Stage__c})`, false},
		{`ROUND({!myVariable_current.Amount__c},2)`, false},
		{`BLANKVALUE({!myVariable_current.Amount__c},0)*7`, false},
		{`IF(ISNEW(),99,1)`, false},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			var pb processBuilderXML
			source := `<Flow><formulas><name>probe</name><expression>` + tc.expression + `</expression></formulas></Flow>`
			if err := xml.Unmarshal([]byte(source), &pb); err != nil {
				t.Fatal(err)
			}
			skip := processBuilderUncoveredOperand(pb, "myVariable_current")
			if (skip == "") != tc.admitted {
				t.Fatalf("skip=%q, admitted=%v", skip, tc.admitted)
			}
		})
	}
}

// One or two ID-filtered, literal-assignment updates are the captured flow
// action shape. Extra update forms and data dependencies stay unrun.
func TestProcessBuilderSubflowEvidenceBoundaries(t *testing.T) {
	update := storage.FlowRecordUpdate{ObjectName: "Cedar__c", Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "recId"}}, InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Memo__c", LiteralValue: "value"}}}
	step := func(u storage.FlowRecordUpdate) storage.FlowStep {
		return storage.FlowStep{Kind: "recordUpdate", RecordUpdate: u}
	}
	input := storage.WorkflowFieldUpdate{Field: "recId", Name: "recId", SourceField: "Id"}
	variable := storage.FlowVariable{Name: "recId", DataType: "String", IsInput: true}
	for _, tc := range []struct {
		name     string
		updates  []storage.FlowRecordUpdate
		inputs   []storage.WorkflowFieldUpdate
		admitted bool
	}{
		{"one", []storage.FlowRecordUpdate{update}, []storage.WorkflowFieldUpdate{input}, true},
		{"two", []storage.FlowRecordUpdate{update, func() storage.FlowRecordUpdate {
			x := update
			x.InputAssignments = []storage.WorkflowFieldUpdate{{Field: "Other__c", LiteralValue: "next"}}
			return x
		}()}, []storage.WorkflowFieldUpdate{input}, true},
		{"three", []storage.FlowRecordUpdate{update, update, update}, []storage.WorkflowFieldUpdate{input}, false},
		{"dynamic_value", []storage.FlowRecordUpdate{func() storage.FlowRecordUpdate {
			x := update
			x.InputAssignments = []storage.WorkflowFieldUpdate{{Field: "Memo__c", Formula: "UPPER(x)"}}
			return x
		}()}, []storage.WorkflowFieldUpdate{input}, false},
		{"unrelated_filter", []storage.FlowRecordUpdate{func() storage.FlowRecordUpdate {
			x := update
			x.Criteria = []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", SourceField: "recId"}}
			return x
		}()}, []storage.WorkflowFieldUpdate{input}, false},
		{"unrelated_input", []storage.FlowRecordUpdate{update}, []storage.WorkflowFieldUpdate{{Field: "recId", SourceField: "Name"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := make([]storage.FlowStep, 0, len(tc.updates))
			for _, u := range tc.updates {
				steps = append(steps, step(u))
			}
			target := storage.FlowRule{Name: "Sub", Active: true, Variables: []storage.FlowVariable{variable}, Steps: steps}
			process := Flow{ObjectName: "Cedar__c", Rules: []storage.FlowRule{{Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "flow", ActionName: "Sub", Inputs: tc.inputs}}}}}}
			defs := map[string]storage.ObjectDefinition{"Cedar__c": {APIName: "Cedar__c", Fields: map[string]storage.Field{"Memo__c": {APIName: "Memo__c", Type: storage.FieldString}, "Other__c": {APIName: "Other__c", Type: storage.FieldString}}}}
			skip := processBuilderUncoveredFlowAction(process, map[string]storage.FlowRule{"sub": target}, defs)
			if (skip == "") != tc.admitted {
				t.Fatalf("skip=%q admitted=%v", skip, tc.admitted)
			}
			if !tc.admitted && !strings.Contains(skip, "flow action") {
				t.Fatalf("non-action reason %q", skip)
			}
		})
	}
}

// The native subflow rows assign Text fields; a number field is a different
// conversion boundary and must not be admitted without its own row.
func TestProcessBuilderSubflowRejectsUncapturedFieldType(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_shapes", "project", "force-app", "main", "default", "flows")
	processPath, subflowPath := filepath.Join(root, "Fern_Flow_Process.flow-meta.xml"), filepath.Join(root, "Fern_Sub_Flow.flow-meta.xml")
	process, _, err := loadFlow(processPath)
	if err != nil {
		t.Fatal(err)
	}
	subflow, _, err := loadFlow(subflowPath)
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string]storage.ObjectDefinition{"Fern_Flow__c": {APIName: "Fern_Flow__c", Fields: map[string]storage.Field{"Note__c": {APIName: "Note__c", Type: storage.FieldDecimal}}}}
	idx := Index{Flows: []Flow{process, subflow}}
	if err := leaveUncoveredProcessesUnrun(&idx, []string{processPath, subflowPath}, defs); err != nil {
		t.Fatal(err)
	}
	if idx.Flows[0].ObjectName != "" {
		t.Fatal("uncaptured decimal assignment was admitted")
	}
}

// Adjacent uncaptured graphs stay gated even though the positive graph has a row.
func TestProcessBuilderCapturedSubflowGraphBoundaries(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review", "project", "force-app", "main", "default", "flows")
	for _, tc := range []struct {
		file  string
		match func(storage.FlowRule) bool
	}{
		{"Fern_Fma_Sub.flow-meta.xml", func(r storage.FlowRule) bool { _, ok := r.CapturedProcessActionSteps(); return ok }},
		{"Fern_Fml_Sub.flow-meta.xml", func(r storage.FlowRule) bool { _, ok := r.CapturedProcessLookupSteps(); return ok }},
	} {
		flow, _, err := loadFlow(filepath.Join(root, tc.file))
		if err != nil {
			t.Fatal(err)
		}
		if len(flow.Rules) != 1 || !tc.match(flow.Rules[0]) {
			t.Fatalf("%s: captured graph not recognized", tc.file)
		}
		rule := flow.Rules[0]
		rule.Steps = append(append([]storage.FlowStep(nil), rule.Steps...), storage.FlowStep{Kind: "assignment"})
		if tc.match(rule) {
			t.Fatalf("%s: extra step admitted", tc.file)
		}
		rule = flow.Rules[0]
		rule.Steps = append([]storage.FlowStep(nil), rule.Steps...)
		rule.Steps[0].FaultTarget = "fault"
		if tc.match(rule) {
			t.Fatalf("%s: fault path admitted", tc.file)
		}
	}
}

// The four-flow capture is a fixed graph boundary; adjacent shapes stay gated.
func TestProcessBuilderFourAfterSaveGraphBoundaries(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review", "project", "force-app", "main", "default", "flows")
	var rules []storage.FlowRule
	for _, name := range []string{"Fern_Faf_A", "Fern_Faf_B", "Fern_Faf_C", "Fern_Faf_D"} {
		flow, _, err := loadFlow(filepath.Join(root, name+".flow-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, flow.Rules...)
	}
	definition := storage.ObjectDefinition{Fields: map[string]storage.Field{"Stage__c": {APIName: "Stage__c", Type: storage.FieldString}, "Note__c": {APIName: "Note__c", Type: storage.FieldString}, "Old__c": {APIName: "Old__c", Type: storage.FieldString}, "Score__c": {APIName: "Score__c", Type: storage.FieldDecimal}, "Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}, "Day__c": {APIName: "Day__c", Type: storage.FieldDate}}}
	definitions := map[string]storage.ObjectDefinition{"Fern_Root__c": {Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}}
	if _, ok := storage.CapturedFourAfterSavePlan(rules, definition, definitions); !ok {
		t.Fatal("captured graph rejected")
	}
	rules[1].Name, rules[2].Name = rules[2].Name, rules[1].Name
	if _, ok := storage.CapturedFourAfterSavePlan(rules, definition, definitions); ok {
		t.Fatal("uncaptured flow name order admitted")
	}
	rules[1].Name, rules[2].Name = rules[2].Name, rules[1].Name
	if _, ok := storage.CapturedFourAfterSavePlan(rules[:3], definition, definitions); ok {
		t.Fatal("three graphs admitted")
	}
	first := &rules[1].Steps[0].Branches[0].Steps[0].Action
	second := &rules[1].Steps[0].Branches[0].Steps[1].Branches[0].Steps[0].Action
	original := second.ActionName
	second.ActionName = first.ActionName
	if _, ok := storage.CapturedFourAfterSavePlan(rules, definition, definitions); ok {
		t.Fatal("repeated action admitted")
	}
	second.ActionName = original

	definition.Fields["Stage__c"] = storage.Field{APIName: "Stage__c", Type: storage.FieldPicklist}
	if _, ok := storage.CapturedFourAfterSavePlan(rules, definition, definitions); ok {
		t.Fatal("uncaptured entry field type admitted")
	}
	definition.Fields["Stage__c"] = storage.Field{APIName: "Stage__c", Type: storage.FieldString}
	definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldString}
	if _, ok := storage.CapturedFourAfterSavePlan(rules, definition, definitions); ok {
		t.Fatal("uncaptured field type admitted")
	}
}

func TestProcessBuilderFormulaRejectsUncapturedUsage(t *testing.T) {
	for _, tc := range []struct{ name, formula, datatype, assignment string }{
		{"boolean_assignment", "false", "Boolean", "Flag__c"},
		{"number_to_text", `BLANKVALUE({!myVariable_current.Note__c},0)+1`, "Number", "Note__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `<Flow><processMetadataValues><name>ObjectType</name><value><stringValue>Cedar__c</stringValue></value></processMetadataValues><formulas><name>probe</name><dataType>` + tc.datatype + `</dataType><expression>` + tc.formula + `</expression></formulas><recordUpdates><object>Cedar__c</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>` + tc.assignment + `</field><value><elementReference>probe</elementReference></value></inputAssignments></recordUpdates></Flow>`
			var pb processBuilderXML
			if err := xml.Unmarshal([]byte(source), &pb); err != nil {
				t.Fatal(err)
			}
			defs := map[string]storage.ObjectDefinition{"Cedar__c": {Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}, "Note__c": {APIName: "Note__c", Type: storage.FieldString}}}}
			if skip := processBuilderUncoveredOperand(pb, "myVariable_current", defs); skip == "" {
				t.Fatal("uncaptured formula usage admitted")
			}
		})
	}
}

func TestProcessBuilderActionGraphRejectsRepeatedAction(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review", "project", "force-app", "main", "default", "flows")
	flow, _, err := loadFlow(filepath.Join(root, "Fern_Fma_Sub.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	r := flow.Rules[0]
	r.Steps = append([]storage.FlowStep(nil), r.Steps...)
	r.Steps[2].Action.ActionName = r.Steps[1].Action.ActionName
	if _, ok := r.CapturedProcessActionSteps(); ok {
		t.Fatal("uncaptured repeated action admitted")
	}
}

func TestProcessBuilderFormulaRejectsUncapturedReferences(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"formula_to_formula", `<decisions><rules><conditions><leftValueReference>probe</leftValueReference><operator>EqualTo</operator><rightValue><elementReference>other</elementReference></rightValue></conditions></rules></decisions><formulas><name>other</name><dataType>Boolean</dataType><expression>true</expression></formulas>`},
		{"record_update_filter", `<recordUpdates><object>Cedar__c</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>probe</elementReference></value></filters></recordUpdates>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `<Flow><formulas><name>probe</name><dataType>Boolean</dataType><expression>true</expression></formulas>` + tc.body + `</Flow>`
			var pb processBuilderXML
			if err := xml.Unmarshal([]byte(source), &pb); err != nil {
				t.Fatal(err)
			}
			if skip := processBuilderUncoveredOperand(pb, "myVariable_current"); skip == "" {
				t.Fatal("uncaptured formula reference admitted")
			}
		})
	}
}

func TestProcessBuilderSpecialFormulaRejectsExtraUse(t *testing.T) {
	defs := map[string]storage.ObjectDefinition{"Cedar__c": {Fields: map[string]storage.Field{"Note__c": {APIName: "Note__c", Type: storage.FieldString}}}}
	for _, tc := range []struct{ name, source string }{
		{"alias_action", `<Flow><formulas><name>probe</name><dataType>Boolean</dataType><expression>NOT(ISBLANK($User.Alias))</expression></formulas><decisions><rules><conditions><leftValueReference>probe</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions></rules></decisions><actionCalls><actionType>apex</actionType><inputParameters><name>x</name><value><elementReference>probe</elementReference></value></inputParameters></actionCalls></Flow>`},
		{"user_filter", `<Flow><formulas><name>probe</name><dataType>String</dataType><expression>$User.Id</expression></formulas><recordUpdates><object>Cedar__c</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>Note__c</field><value><elementReference>probe</elementReference></value></inputAssignments></recordUpdates><recordUpdates><object>Cedar__c</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>probe</elementReference></value></filters></recordUpdates></Flow>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pb processBuilderXML
			if err := xml.Unmarshal([]byte(tc.source), &pb); err != nil {
				t.Fatal(err)
			}
			if skip := processBuilderUncoveredOperand(pb, "myVariable_current", defs); skip == "" {
				t.Fatal("uncaptured special-formula use admitted")
			}
		})
	}
}

func TestProcessBuilderRoutedUpdateRejectsUncapturedTypes(t *testing.T) {
	filters := `<filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters>`
	for _, value := range []string{"a", "b", "c", "d"} {
		filters += `<filters><field>Mode__c</field><operator>EqualTo</operator><value><stringValue>` + value + `</stringValue></value></filters>`
	}
	source := `<Flow><recordUpdates><object>Cedar__c</object><filterLogic>1 AND (2 OR 3 OR 4 OR 5)</filterLogic>` + filters + `<inputAssignments><field>Note__c</field><value><stringValue>changed</stringValue></value></inputAssignments></recordUpdates></Flow>`
	for _, tc := range []struct {
		name                   string
		filterType, updateType storage.FieldType
	}{{"filter_picklist", storage.FieldPicklist, storage.FieldString}, {"assignment_decimal", storage.FieldString, storage.FieldDecimal}, {"different_picklists", storage.FieldPicklist, storage.FieldPicklist}} {
		t.Run(tc.name, func(t *testing.T) {
			var pb processBuilderXML
			if err := xml.Unmarshal([]byte(source), &pb); err != nil {
				t.Fatal(err)
			}
			defs := map[string]storage.ObjectDefinition{"Cedar__c": {Fields: map[string]storage.Field{"Mode__c": {APIName: "Mode__c", Type: tc.filterType}, "Note__c": {APIName: "Note__c", Type: tc.updateType}}}}
			if skip := processBuilderUncoveredOperand(pb, "myVariable_current", defs); skip == "" {
				t.Fatal("uncaptured routed field type admitted")
			}
		})
	}
}

func TestProcessBuilderRecordSubflowBoundaries(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_record", "project", "force-app", "main", "default", "flows")
	flow, _, err := loadFlow(filepath.Join(root, "Fern_Record_Sub.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	target := flow.Rules[0]
	action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Field: "rec", SourceField: "$Record"}}}
	defs := map[string]storage.ObjectDefinition{"Account": {Fields: map[string]storage.Field{"Description": {APIName: "Description", Type: storage.FieldString, DisplayType: "TEXTAREA"}}}}
	if !processBuilderCapturedSubflow("Account", action, target, defs) {
		t.Fatal("captured graph rejected")
	}
	bad := action
	bad.Inputs = []storage.WorkflowFieldUpdate{{Field: "rec", SourceField: "Id"}}
	if processBuilderCapturedSubflow("Account", bad, target, defs) {
		t.Fatal("scalar input admitted")
	}
	defs["Account"] = storage.ObjectDefinition{Fields: map[string]storage.Field{"Description": {APIName: "Description", Type: storage.FieldDecimal}}}
	if processBuilderCapturedSubflow("Account", action, target, defs) {
		t.Fatal("uncaptured field type admitted")
	}
}

func TestProcessBuilderDependentLookupBoundaries(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_lookup", "project", "force-app", "main", "default", "flows")
	flow, _, err := loadFlow(filepath.Join(root, "Fern_Lookup_Sub.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	target := flow.Rules[0]
	action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Field: "recordId", SourceField: "Id"}, {Field: "rootId", SourceField: "Fern_Root_Link__c"}}}
	defs := map[string]storage.ObjectDefinition{"Contact": {Fields: map[string]storage.Field{"Fern_Root_Link__c": {APIName: "Fern_Root_Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Fern_Root__c"}}, "Description": {APIName: "Description", Type: storage.FieldString, DisplayType: "TEXTAREA"}}}, "Fern_Root__c": {Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}}
	if !processBuilderCapturedSubflow("Contact", action, target, defs) {
		t.Fatal("captured dependent lookup rejected")
	}
	bad := action
	bad.Inputs = append([]storage.WorkflowFieldUpdate(nil), action.Inputs...)
	bad.Inputs[1].SourceField = "Name"
	if processBuilderCapturedSubflow("Contact", bad, target, defs) {
		t.Fatal("uncaptured binding admitted")
	}
	d := defs["Contact"]
	d.Fields["Fern_Root_Link__c"] = storage.Field{APIName: "Fern_Root_Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Other__c"}}
	defs["Contact"] = d
	if processBuilderCapturedSubflow("Contact", action, target, defs) {
		t.Fatal("unrelated lookup target admitted")
	}
}

func TestProcessBuilderDependentLookupHTMLBoundary(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_html", "project", "force-app", "main", "default", "flows")
	flow, _, err := loadFlow(filepath.Join(root, "Fern_Lookup_Sub.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	target := flow.Rules[0]
	action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Field: "recordId", SourceField: "Id"}, {Field: "rootId", SourceField: "Fern_Root_Link__c"}}}
	defs := map[string]storage.ObjectDefinition{"Contact": {Fields: map[string]storage.Field{"Fern_Root_Link__c": {APIName: "Fern_Root_Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Fern_Root__c"}}, "Fern_Result__c": {APIName: "Fern_Result__c", Type: storage.FieldString, DisplayType: "RICHTEXTAREA"}}}, "Fern_Root__c": {Fields: map[string]storage.Field{"Fern_Value__c": {APIName: "Fern_Value__c", Type: storage.FieldString, DisplayType: "TEXTAREA", LongTextArea: true}}}}
	if !processBuilderCapturedSubflow("Contact", action, target, defs) {
		t.Fatal("captured Long Text to HTML rejected")
	}
	d := defs["Contact"]
	d.Fields["Fern_Result__c"] = storage.Field{APIName: "Fern_Result__c", Type: storage.FieldString, DisplayType: "TEXTAREA", LongTextArea: true}
	defs["Contact"] = d
	if processBuilderCapturedSubflow("Contact", action, target, defs) {
		t.Fatal("uncaptured Long Text to Long Text admitted")
	}
}

func TestProcessBuilderFormatFormulaBoundaries(t *testing.T) {
	source := `<Flow><formulas><name>fmt</name><dataType>String</dataType><expression>TEXT({!myVariable_current.Choice__c}) + " - " + TEXT(DATEVALUE({!myVariable_current.Stamp__c}))</expression></formulas><recordUpdates><object>Cedar__c</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>Note__c</field><value><elementReference>fmt</elementReference></value></inputAssignments></recordUpdates></Flow>`
	var pb processBuilderXML
	if err := xml.Unmarshal([]byte(source), &pb); err != nil {
		t.Fatal(err)
	}
	def := storage.ObjectDefinition{Fields: map[string]storage.Field{"Choice__c": {APIName: "Choice__c", Type: storage.FieldPicklist}, "Stamp__c": {APIName: "Stamp__c", Type: storage.FieldDateTime}, "Note__c": {APIName: "Note__c", Type: storage.FieldString, DisplayType: "STRING"}}}
	defs := map[string]storage.ObjectDefinition{"Cedar__c": def}
	if !processBuilderCapturedFormatFormula(pb, "fmt", "String", pb.Formulas[0].Expression, "myVariable_current", defs) {
		t.Fatal("captured formula rejected")
	}
	def.Fields["Choice__c"] = storage.Field{APIName: "Choice__c", Type: storage.FieldString}
	if processBuilderCapturedFormatFormula(pb, "fmt", "String", pb.Formulas[0].Expression, "myVariable_current", defs) {
		t.Fatal("uncaptured source type admitted")
	}
	def.Fields["Choice__c"] = storage.Field{APIName: "Choice__c", Type: storage.FieldPicklist}
	def.Fields["Stamp__c"] = storage.Field{APIName: "Stamp__c", Type: storage.FieldDate}
	if processBuilderCapturedFormatFormula(pb, "fmt", "String", pb.Formulas[0].Expression, "myVariable_current", defs) {
		t.Fatal("uncaptured date source type admitted")
	}
}

// Already-unrun processes have no runtime object and must not be counted as
// several processes on a fictitious empty-named object.
func TestProcessBuilderUnrunProcessesDoNotShareEmptyObject(t *testing.T) {
	base := filepath.Join("..", "apextest", "testdata", "conformance")
	paths := []string{filepath.Join(base, "process_builder_review_format", "project", "force-app", "main", "default", "flows", "Fern_Fmt_Process.flow-meta.xml"), filepath.Join(base, "process_builder_review_prior", "project", "force-app", "main", "default", "flows", "Fern_Record_Process.flow-meta.xml")}
	var idx Index
	for _, path := range paths {
		idx.Flows = append(idx.Flows, Flow{File: path, Rules: []storage.FlowRule{{Name: filepath.Base(path), Active: true, ProcessType: "Workflow"}}})
	}
	before := len(idx.Diagnostics)
	if err := leaveUncoveredProcessesUnrun(&idx, paths, nil); err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != before {
		t.Fatalf("cross-load added %d warnings for empty objects", len(idx.Diagnostics)-before)
	}
}

func TestProcessBuilderPriorBlankFormulaBoundaries(t *testing.T) {
	source := `<Flow><processMetadataValues><name>ObjectType</name><value><stringValue>Account</stringValue></value></processMetadataValues><formulas><name>probe</name><dataType>Boolean</dataType><expression>IF(ISBLANK({!myVariable_current.Fern_Text__c}), TRUE, PRIORVALUE({!myVariable_current.Name}) &lt;&gt; {!myVariable_current.Name})</expression></formulas><decisions><rules><conditions><leftValueReference>probe</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions></rules></decisions></Flow>`
	var pb processBuilderXML
	if err := xml.Unmarshal([]byte(source), &pb); err != nil {
		t.Fatal(err)
	}
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Fern_Text__c": {APIName: "Fern_Text__c", Type: storage.FieldString, DisplayType: "STRING"}}}
	defs := map[string]storage.ObjectDefinition{"Account": def}
	expr := pb.Formulas[0].Expression
	if !processBuilderCapturedPriorBlankFormula(pb, "probe", "Boolean", expr, "myVariable_current", defs) {
		t.Fatal("captured decision rejected")
	}
	changed := strings.Replace(expr, "{!myVariable_current.Name})", "{!myVariable_current.Phone})", 1)
	if processBuilderCapturedPriorBlankFormula(pb, "probe", "Boolean", changed, "myVariable_current", defs) {
		t.Fatal("different compared field admitted")
	}
	personExpr := strings.ReplaceAll(expr, "myVariable_current.Name", "myVariable_current.PersonMailingCity")
	if processBuilderCapturedPriorBlankFormula(pb, "probe", "Boolean", personExpr, "myVariable_current", defs) {
		t.Fatal("Person Account field admitted without its feature")
	}
	def.Metadata = map[string]string{processBuilderPersonAccountsFeature: "true"}
	defs["Account"] = def
	if !processBuilderCapturedPriorBlankFormula(pb, "probe", "Boolean", personExpr, "myVariable_current", defs) {
		t.Fatal("captured Person Account field rejected")
	}
	def.Fields["Fern_Text__c"] = storage.Field{APIName: "Fern_Text__c", Type: storage.FieldBoolean}
	defs["Account"] = def
	if processBuilderCapturedPriorBlankFormula(pb, "probe", "Boolean", expr, "myVariable_current", defs) {
		t.Fatal("uncaptured blank-field type admitted")
	}
}

func TestProcessBuilderWrappedUserFormulaBoundaries(t *testing.T) {
	source := `<Flow><formulas><name>who</name><dataType>String</dataType><expression>{!$User.Id}</expression></formulas><formulas><name>when</name><dataType>DateTime</dataType><expression>NOW()</expression></formulas><recordUpdates><object>Case</object><filters><field>Id</field><operator>EqualTo</operator><value><elementReference>myVariable_current.Id</elementReference></value></filters><inputAssignments><field>Actor__c</field><value><elementReference>who</elementReference></value></inputAssignments><inputAssignments><field>When__c</field><value><elementReference>when</elementReference></value></inputAssignments></recordUpdates></Flow>`
	var pb processBuilderXML
	if err := xml.Unmarshal([]byte(source), &pb); err != nil {
		t.Fatal(err)
	}
	def := storage.ObjectDefinition{Fields: map[string]storage.Field{"Actor__c": {APIName: "Actor__c", Type: storage.FieldReference, ReferenceTo: []string{"User"}}, "When__c": {APIName: "When__c", Type: storage.FieldDateTime}}}
	defs := map[string]storage.ObjectDefinition{"Case": def}
	if !processBuilderCapturedValueResource(pb, "who", "String", "{!$User.Id}", "myVariable_current", defs) {
		t.Fatal("captured wrapped assignment rejected")
	}
	pb.RecordUpdates[0].InputAssignments = pb.RecordUpdates[0].InputAssignments[:1]
	if !processBuilderCapturedValueResource(pb, "who", "String", "{!$User.Id}", "myVariable_current", defs) {
		t.Fatal("captured single assignment rejected")
	}
	def.Fields["Actor__c"] = storage.Field{APIName: "Actor__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	if processBuilderCapturedValueResource(pb, "who", "String", "{!$User.Id}", "myVariable_current", defs) {
		t.Fatal("non-User lookup admitted")
	}
}

// Only the captured dependent two-process Text graph runs; different graph
// shapes stay gated even when both processes happen to name the same object.
func TestProcessBuilderDependentPairBoundaries(t *testing.T) {
	base := "../apextest/testdata/conformance/process_builder_review_multiprocess/project/force-app/main/default/flows/"
	first, err := os.ReadFile(base + "Fern_Mpr_A.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(base + "Fern_Mpr_B.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]storage.Field{}
	for _, name := range []string{"Stage__c", "Note__c", "Old__c"} {
		fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DisplayType: "STRING"}
	}
	defs := map[string]storage.ObjectDefinition{"Fern_Mpr__c": {APIName: "Fern_Mpr__c", Fields: fields}}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.xml"), filepath.Join(dir, "b.xml")
	for _, tc := range []struct {
		name          string
		first, second string
		accepted      bool
	}{
		{"captured", string(first), string(second), true},
		{"unrelated criterion", string(first), strings.Replace(string(second), "myVariable_current.Note__c", "myVariable_current.Stage__c", 1), false},
		{"different comparison", string(first), strings.Replace(string(second), "<stringValue>A</stringValue>", "<stringValue>C</stringValue>", 1), false},
		{"additional update", string(first), strings.Replace(string(second), "</recordUpdates>", "<connector><targetReference>other</targetReference></connector></recordUpdates>", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(a, []byte(tc.first), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(b, []byte(tc.second), 0600); err != nil {
				t.Fatal(err)
			}
			if got := processBuilderCapturedPair("Fern_Mpr__c", []string{a, b}, defs); got != tc.accepted {
				t.Fatalf("accepted = %v, want %v", got, tc.accepted)
			}
		})
	}
	if err := os.WriteFile(a, first, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, second, 0600); err != nil {
		t.Fatal(err)
	}
	fields["Old__c"] = storage.Field{APIName: "Old__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	if processBuilderCapturedPair("Fern_Mpr__c", []string{a, b}, defs) {
		t.Fatal("non-Text output admitted")
	}
}
