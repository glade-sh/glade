package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The source65 named observation distinguishes User-owned, Group-owned and
// absent lookup rows for a typed relationship Id. Relationship metadata, not
// an owner-field spelling or a hard-coded key prefix, supplies this fixture.
func TestFlowFormulaTypedRelationshipIDContext(t *testing.T) {
	for _, selected := range []struct {
		name, ownerType string
		missing         bool
		wantNull        bool
	}{
		{name: "matchingUser", ownerType: "User"},
		{name: "otherAllowedGroup", ownerType: "Group", wantNull: true},
		{name: "missingLookup", missing: true, wantNull: true},
	} {
		t.Run(selected.name, func(t *testing.T) {
			engine, frame, ownerID := flowFormulaContextFixture(t, selected.ownerType, selected.missing)
			value, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: "Fetched.Owner:User.Id"}, frame)
			if !ok {
				t.Fatal("typed relationship formula was not resolved")
			}
			if selected.wantNull {
				if value.Kind != storage.ValueNull {
					t.Fatalf("typed relationship value = %#v, want null", value)
				}
				return
			}
			if value.Kind != storage.ValueString || value.String != string(ownerID) {
				t.Fatalf("typed relationship value = %#v, want matching reference %s", value, ownerID)
			}
		})
	}
}

func TestFlowFormulaTypedRelationshipCurrentUserFallback(t *testing.T) {
	for _, selected := range []struct {
		name, ownerType string
		missing         bool
		selectOwner     bool
	}{
		{name: "matchingUser", ownerType: "User", selectOwner: true},
		{name: "otherAllowedGroup", ownerType: "Group"},
		{name: "missingLookup", missing: true},
	} {
		t.Run(selected.name, func(t *testing.T) {
			engine, frame, ownerID := flowFormulaContextFixture(t, selected.ownerType, selected.missing)
			want := string(engine.UserID)[:15]
			if selected.selectOwner {
				want = string(ownerID)
			}
			value, ok := engine.flowFrameValue(storage.FlowAssignment{
				Formula: "IF(Fetched.Owner:User.Id != null, Fetched.Owner:User.Id, $User.Id)",
			}, frame)
			if !ok || value.Kind != storage.ValueString || value.String != want {
				t.Fatalf("selected value = %#v, resolved=%v; want %q", value, ok, want)
			}
		})
	}
}

// Native source65 refined observation 0Afdh00000AbMtSCAV returns the
// 15-character current-user String through direct and formula routes, while
// the UserInfo ID is 18 characters and both cast to the same Id identity.
func TestFlowFormulaCurrentUserContextNonNull(t *testing.T) {
	for _, assignment := range []struct {
		name  string
		value storage.FlowAssignment
	}{
		{name: "formula", value: storage.FlowAssignment{Formula: "$User.Id"}},
		{name: "directReference", value: storage.FlowAssignment{SourceField: "$User.Id"}},
	} {
		t.Run(assignment.name, func(t *testing.T) {
			engine, frame, _ := flowFormulaContextFixture(t, "User", false)
			value, ok := engine.flowFrameValue(assignment.value, frame)
			want := string(engine.UserID)[:15]
			if !ok || value.Kind != storage.ValueString || value.String != want || !storage.IDsEqual(storage.ID(value.String), engine.UserID) {
				t.Fatalf("current-user value = %#v, resolved=%v; want 15-character String %q with matching Id identity", value, ok, want)
			}
		})
	}
}

// Native source65 retains an empty input String while comparing it equal to
// null. A null input remains null; either differs from nonempty text.
func TestFlowFormulaTextNullComparisons(t *testing.T) {
	for _, input := range []struct {
		name     string
		value    storage.Value
		nonempty bool
	}{
		{"nonempty", storage.StringValue("owned"), true},
		{"null", storage.NullValue(), false},
		{"empty", storage.StringValue(""), false},
	} {
		t.Run(input.name, func(t *testing.T) {
			engine, frame, _ := flowFormulaContextFixture(t, "User", false)
			frame.scalars[flowFrameKey("TextInput")] = input.value
			for _, comparison := range []struct {
				expression string
				want       bool
			}{
				{"TextInput != null", input.nonempty},
				{"TextInput = null", !input.nonempty},
				{`TextInput != "owned"`, !input.nonempty},
			} {
				value, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: comparison.expression}, frame)
				if !ok || value.Kind != storage.ValueBoolean || value.Boolean != comparison.want {
					t.Errorf("%s = %#v, resolved=%v; want %v", comparison.expression, value, ok, comparison.want)
				}
			}
			if got := frame.scalars[flowFrameKey("TextInput")]; got.Kind != input.value.Kind || got.String != input.value.String {
				t.Fatalf("comparison changed input: %#v", got)
			}
		})
	}
}

func TestFlowFormulaContextKeepsExistingAssignments(t *testing.T) {
	engine, frame, ownerID := flowFormulaContextFixture(t, "User", false)
	frame.scalars[flowFrameKey("TextOperand")] = storage.StringValue("kept")
	for _, selected := range []struct {
		name       string
		assignment storage.FlowAssignment
		want       string
	}{
		{name: "ordinaryOwnerID", assignment: storage.FlowAssignment{Formula: "Fetched.OwnerId"}, want: string(ownerID)},
		{name: "scalarFormula", assignment: storage.FlowAssignment{Formula: "TextOperand"}, want: "kept"},
		{name: "colonLiteral", assignment: storage.FlowAssignment{Formula: `"Fetched.Owner:User.Id"`}, want: "Fetched.Owner:User.Id"},
		{name: "globalLiteral", assignment: storage.FlowAssignment{Formula: `"$User.Id"`}, want: "$User.Id"},
	} {
		t.Run(selected.name, func(t *testing.T) {
			value, ok := engine.flowFrameValue(selected.assignment, frame)
			if !ok || value.Kind != storage.ValueString || value.String != selected.want {
				t.Fatalf("value = %#v, resolved=%v; want %q", value, ok, selected.want)
			}
		})
	}

	// F14_021/F14_022 own non-null String collection Add; the scalar concat
	// controls are owned by F14_013/F14_014. Preserve both existing routes.
	frame.variables[flowFrameKey("Items")] = storage.FlowVariable{Name: "Items", DataType: "String", IsCollection: true}
	frame.scalars[flowFrameKey("Items")] = storage.ListValue()
	if err := engine.applyFlowAssignmentStep(storage.FlowAssignment{Name: "Append", Target: "Items", Operator: "Add", LiteralValue: "tail"}, frame); err != nil {
		t.Fatal(err)
	}
	items := frame.scalars[flowFrameKey("Items")]
	if items.Kind != storage.ValueList || len(items.List) != 1 || items.List[0].Kind != storage.ValueString || items.List[0].String != "tail" {
		t.Fatalf("primitive Add changed: %#v", items)
	}
	frame.variables[flowFrameKey("Text")] = storage.FlowVariable{Name: "Text", DataType: "String"}
	frame.scalars[flowFrameKey("Text")] = storage.StringValue("base")
	if err := engine.applyFlowAssignmentStep(storage.FlowAssignment{Name: "Concat", Target: "Text", Operator: "Add", LiteralValue: "tail"}, frame); err != nil {
		t.Fatal(err)
	}
	if text := frame.scalars[flowFrameKey("Text")]; text.Kind != storage.ValueString || text.String != "basetail" {
		t.Fatalf("scalar Add changed: %#v", text)
	}
}

// These local controls retain the pre-existing unsupported boundary. They do
// not claim native rejection for unobserved relationship or traversal shapes.
func TestFlowFormulaContextKeepsUnsupportedTypedReferences(t *testing.T) {
	for _, expression := range []string{
		"Fetched.Absent:User.Id",
		"Fetched.Owner:UnknownTarget.Id",
		"Fetched.Owner:User.Name",
		"Fetched.Owner:.Id",
		"Fetched.Owner::User.Id",
		"Fetched.Owner:User.Id:User.Id",
	} {
		t.Run(expression, func(t *testing.T) {
			engine, frame, _ := flowFormulaContextFixture(t, "User", false)
			if value, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: expression}, frame); ok {
				t.Fatalf("unsupported typed shape resolved to %#v", value)
			}
		})
	}
}

// Preserve the existing interpreter's lazy IF boundary, including resources
// that cannot be resolved when their branch is not evaluated.
func TestFlowFormulaContextKeepsLazyBranches(t *testing.T) {
	engine, frame, _ := flowFormulaContextFixture(t, "User", false)
	engine.UserID = "unavailable"
	for _, expression := range []string{
		`IF(true,"kept",Fetched.Absent:User.Id)`,
		`IF(false,Fetched.Absent:User.Id,"kept")`,
		`IF(true,"kept",$User.Id)`,
		`IF(false,$User.Id,"kept")`,
	} {
		value, ok := engine.flowFrameValue(storage.FlowAssignment{Formula: expression}, frame)
		if !ok || value.Kind != storage.ValueString || value.String != "kept" {
			t.Errorf("%s = %#v, resolved=%v; want kept", expression, value, ok)
		}
	}
}

func TestAutolaunchedFlowPrimitiveAssignmentDeclaration(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	rule := storage.FlowRule{Name: "OwnedAppend", Variables: []storage.FlowVariable{
		{Name: "Items", DataType: "String", IsCollection: true, IsOutput: true},
	}}
	control, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
	if err != nil || control.Scalars[flowFrameKey("Items")].Kind != storage.ValueList || len(control.Scalars[flowFrameKey("Items")].List) != 0 {
		t.Fatalf("unassigned collection control = %#v, %v", control, err)
	}
	rule.Steps = []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{
		Name: "Append", Target: "Items", Operator: "Add", LiteralValue: "owned",
	}}}
	output, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
	if err != nil {
		t.Fatal(err)
	}
	items := output.Scalars[flowFrameKey("Items")]
	if items.Kind != storage.ValueList || len(items.List) != 1 || items.List[0].Kind != storage.ValueString || items.List[0].String != "owned" {
		t.Fatalf("primitive interview Add = %#v", items)
	}
}

func TestFlowStoredOwnerReferenceProjection(t *testing.T) {
	for _, ownerType := range []string{"User", "Group"} {
		t.Run(ownerType, func(t *testing.T) {
			engine, frame, ownerID := flowFormulaContextFixture(t, ownerType, false)
			output := frame.lookupOutputs[flowFrameKey("Fetched")]
			// Canonical persisted records keep the standard OwnerId in System.
			output.record.System.OwnerID = ownerID
			delete(output.record.Fields, "OwnerId")
			frame.lookupOutputs[flowFrameKey("Fetched")] = output
			value, ok := engine.flowFrameValue(storage.FlowAssignment{SourceField: "Fetched.OwnerId"}, frame)
			if !ok || idFromStorageValue(value) != ownerID {
				t.Fatalf("stored owner projection = %#v, resolved=%v; want %s", value, ok, ownerID)
			}
			value, ok = engine.flowFrameValue(storage.FlowAssignment{Formula: "Fetched.Owner:User.Id"}, frame)
			if !ok || ownerType == "User" && (value.Kind != storage.ValueString || value.String != string(ownerID)) || ownerType == "Group" && value.Kind != storage.ValueNull {
				t.Fatalf("stored typed owner projection = %#v, resolved=%v", value, ok)
			}
			// Preserve the pre-existing explicit-null projection even when a
			// local frame also carries stale system-slot data.
			output.record.ExplicitNulls = map[string]bool{"OwnerId": true}
			frame.lookupOutputs[flowFrameKey("Fetched")] = output
			value, ok = engine.flowFrameValue(storage.FlowAssignment{SourceField: "Fetched.OwnerId"}, frame)
			if !ok || value.Kind != storage.ValueNull {
				t.Fatalf("explicit-null owner projection = %#v, resolved=%v", value, ok)
			}
		})
	}
}

func flowFormulaContextFixture(t *testing.T, ownerType string, missing bool) (*Engine, *flowFrame, storage.ID) {
	t.Helper()
	org := storage.NewOrgState()
	for _, name := range []string{"Case", "User", "Group"} {
		definition, ok := storage.StandardObjectDefinition(name)
		if !ok || definition.KeyPrefix == "" {
			t.Fatalf("missing owned standard definition for %s", name)
		}
		org.Objects[name] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
	}
	definition := org.Objects["Case"].Definition
	definition.Fields["OwnerId"] = storage.Field{
		APIName: "OwnerId", Type: storage.FieldReference, RelationshipName: "Owner", ReferenceTo: []string{"User", "Group"},
	}
	org.Objects["Case"] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
	userID := storage.ID(org.Objects["User"].Definition.KeyPrefix + "000000000001AAA")
	ownerID := userID
	if ownerType != "" {
		ownerID = storage.ID(org.Objects[ownerType].Definition.KeyPrefix + "000000000002AAA")
	}
	engine := NewEngine(&org)
	engine.UserID = userID
	frame := newFlowFrame()
	record := storage.Record{}
	if !missing {
		record = storage.Record{Object: "Case", Fields: map[string]storage.Value{"OwnerId": storage.IDValue(ownerID)}}
	}
	frame.lookupOutputs[flowFrameKey("Fetched")] = flowLookupOutput{record: record, definition: definition}
	return &engine, frame, ownerID
}
