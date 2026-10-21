package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Existing owned API62/63/67 named and anonymous observations distinguish a
// JSON null Datetime root from nullable fields and collection entries.
func TestJSONDeserializeDatetimeRootNull(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		for _, named := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/named=%t", api, named), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
String observed='returned';
try {Datetime value=(Datetime)JSON.deserialize('null',Datetime.class);}
catch(Exception problem) {observed=problem.getTypeName()+'|'+problem.getMessage();}
System.assertEquals('System.JSONException|Invalid format: "null"',observed);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if named {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// These are preservation controls, not native claims for uncaptured versions,
// namespaces or source classes. Nullable recursive paths have native controls.
func TestJSONDatetimeNullKeepsUnobservedOrigins(t *testing.T) {
	for _, cell := range []struct {
		name, api, namespace string
		root, sourceClass    bool
	}{
		{name: "source61", api: "61.0", root: true},
		{name: "source68", api: "68.0", root: true},
		{name: "unknown-source", root: true},
		{name: "recursive", api: "67.0"},
		{name: "source-class", api: "67.0", root: true, sourceClass: true},
		{name: "namespace", api: "67.0", root: true, namespace: "Owned"},
	} {
		for _, raw := range []any{nil, "null"} {
			t.Run(fmt.Sprintf("%s/%v", cell.name, raw), func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod = Method{APIVersion: cell.api}
				if cell.namespace != "" {
					machine.Org = &storage.OrgState{Namespace: cell.namespace}
				}
				if cell.sourceClass {
					if err := machine.RegisterClass(Class{Name: "Datetime", APIVersion: cell.api}); err != nil {
						t.Fatal(err)
					}
				}
				value, err := machine.typedValueFromJSON("Datetime", jsonTypedInput{value: raw, root: cell.root}, false)
				_, _, priorErr := typedScalarFromJSON("Datetime", raw)
				if value.Kind != ValueNull || !sameJSONDatetimeControlError(err, priorErr) {
					t.Fatalf("unobserved origin changed: %v; prior=%v", err, priorErr)
				}
			})
		}
	}
	// The Date owner keeps its independent root conversion and version bounds.
	machine := New(nil)
	machine.currentMethod = Method{APIVersion: "67.0"}
	if err := machine.jsonRootDatetimeNullError("Date", jsonTypedInput{root: true}); err != nil {
		t.Fatalf("Date conversion intercepted: %v", err)
	}
}

func TestJSONDatetimeNullKeepsUncapturedParserPositions(t *testing.T) {
	for _, cell := range []struct {
		name, document string
		index          int64
		raw            any
	}{
		{name: "unstarted", document: "null", index: -1},
		{name: "nested", document: `{"stamp":null}`, index: 2},
		{name: "quoted", document: `"null"`, index: 0, raw: "null"},
	} {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/strict=%t", cell.name, strict), func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod = Method{APIVersion: "67.0"}
				parser, err := newJSONParser(cell.document)
				if err != nil {
					t.Fatal(err)
				}
				parser.Fields["index"] = Int(cell.index)
				target := Object("Type")
				target.Fields["value"] = String("Datetime")
				value, _, err := machine.jsonParserReadValueAs(parser, target, strict)
				_, _, priorErr := typedScalarFromJSON("Datetime", cell.raw)
				if value.Kind != ValueNull || !sameJSONDatetimeControlError(err, priorErr) {
					t.Fatalf("unprobed parser position changed: %v; prior=%v", err, priorErr)
				}
			})
		}
	}
}

func sameJSONDatetimeControlError(got, want error) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return got.Error() == want.Error()
}
