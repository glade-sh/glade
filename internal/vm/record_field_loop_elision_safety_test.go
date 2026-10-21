package vm

import "testing"

func TestRecordFieldLoopElisionFallsBackForSplitAliases(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AliasSplitNode", Fields: map[string]Field{
		"Value": {Name: "Value", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	target := Object("AliasSplitNode")
	target.Fields["Value"] = String("before")
	clone := machine.cloneValuePreserveRefs(target)
	machine.Globals["source"] = target
	machine.Globals["rows"] = List(clone)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	if err := machine.assignPath(target, []string{"Value"}, String("after")); err != nil {
		t.Fatal(err)
	}
	if got := machine.Globals["rows"].List[0].Fields["Value"]; got.Kind != ValueString || got.Text != "after" {
		t.Fatalf("split alias Value = %#v, want after", got)
	}
	if got := recorder.Snapshot().ScopeAlias.ElisionFallbackWalks; got != 1 {
		t.Fatalf("elision fallback walks = %d, want 1", got)
	}
}

func TestRecordFieldLoopElisionKeepsNilFieldsWriteback(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AliasNilFieldsNode", Fields: map[string]Field{
		"Value": {Name: "Value", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	target := Object("AliasNilFieldsNode")
	target.Fields = nil
	machine.Globals["alias"] = target
	if err := machine.assignPath(target, []string{"Value"}, String("after")); err != nil {
		t.Fatal(err)
	}
	if got := machine.Globals["alias"].Fields["Value"]; got.Kind != ValueString || got.Text != "after" {
		t.Fatalf("nil-Fields alias Value = %#v, want after", got)
	}
}

func TestRecordFieldLoopElisionKeepsStaticPropagation(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AliasStaticNode", Fields: map[string]Field{
		"Value": {Name: "Value", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "AliasStaticRegistry", StaticFields: map[string]Field{}}); err != nil {
		t.Fatal(err)
	}
	target := Object("AliasStaticNode")
	target.Fields["Value"] = String("before")
	machine.Globals["source"] = target
	class := machine.Classes["AliasStaticRegistry"]
	class.StaticFields["Shared"] = Field{Name: "Shared", Type: "AliasStaticNode", Static: true, Value: target}
	machine.Classes["AliasStaticRegistry"] = class
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	if err := machine.assignPath(target, []string{"Value"}, String("after")); err != nil {
		t.Fatal(err)
	}
	stats := recorder.Snapshot()
	if stats.ScopeAlias.Calls != 0 {
		t.Fatalf("same-backing write made %d scope calls, want 0", stats.ScopeAlias.Calls)
	}
	if stats.StaticAlias.Calls == 0 {
		t.Fatal("same-backing write skipped static alias propagation")
	}
	if got := machine.Classes["AliasStaticRegistry"].StaticFields["Shared"].Value.Fields["Value"]; got.Kind != ValueString || got.Text != "after" {
		t.Fatalf("static alias Value = %#v, want after", got)
	}
}
