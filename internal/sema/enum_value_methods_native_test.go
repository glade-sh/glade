package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testutil/enumvaluefixture"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestEnumValueMethodsNativeNamedIsTestAnonymousRoutes(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Boundary != "" {
			continue
		} // Native rejects the local enum declaration before any method call.
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			paths, _ := enumvaluefixture.WriteProject(t, root, fixture, row)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			if row.Case.Route == "anonymous" {
				result = AnalyzeAnonymous(index, row.Case.Source, row.API)
			}
			enumvaluefixture.AssertDiagnostics(t, row, result.Diagnostics)
		})
	}
}

// Exercise the call checker directly as well as the project and editor gates.
func TestEnumValueMethodsNativeIRCallRoute(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route != "named" || row.Case.Method != "unknown" {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			paths, target := enumvaluefixture.WriteProject(t, root, fixture, row)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			model := buildSemaTypeMemberView(index)
			typ := typesys.TypeSymbol{Name: row.Case.Name, File: target, EffectiveAPIVersion: row.API}
			member := typesys.MemberSymbol{Name: "probe", Kind: apexast.DeclarationMethod, Type: "void", Modifiers: []string{"static"}}
			expr := ir.Expr{Kind: ir.ExprCall, Callee: strings.TrimSuffix(row.Case.Expression, "()")}
			diagnostics := NewAnalyzer().checkIRCall(typ, member, expr, newIRSemaScope(nil), 0, 0, row.Case.Source, model, nil)
			enumvaluefixture.AssertDiagnostics(t, row, diagnostics)
		})
	}
}

// Only captured arities (values(), valueOf(String)) get the native static-method
// diagnostic. Other arities have no native row, so the checker does not claim them.
func TestEnumValueMethodsNativeWrongArityNotClaimed(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route != "named" || row.Compiled || (row.Case.Method != "values_on_value" && row.Case.Method != "valueOf_on_value") {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			paths, target := enumvaluefixture.WriteProject(t, root, fixture, row)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: paths}, schema.Schema{})
			model := buildSemaTypeMemberView(index)
			typ := typesys.TypeSymbol{Name: row.Case.Name, File: target, EffectiveAPIVersion: row.API}
			member := typesys.MemberSymbol{Name: "probe", Kind: apexast.DeclarationMethod, Type: "void", Modifiers: []string{"static"}}
			callee, _, _ := strings.Cut(row.Case.Expression, "(")
			receiver, method, _ := splitSemaMethodPath(callee)
			good, bad := []ir.Expr{}, []ir.Expr{{}}
			if strings.EqualFold(method, "valueOf") {
				good, bad = bad, good
			}
			if _, handled := NewAnalyzer().checkIREnumConstantCall(typ, member, receiver, method, good, newIRSemaScope(nil), 0, 0, row.Case.Source, model); !handled {
				t.Fatalf("org.tsv:%d captured arity not handled", row.Line)
			}
			if diagnostics, handled := NewAnalyzer().checkIREnumConstantCall(typ, member, receiver, method, bad, newIRSemaScope(nil), 0, 0, row.Case.Source, model); handled {
				t.Fatalf("org.tsv:%d uncaptured arity claimed: %+v", row.Line, diagnostics)
			}
		})
	}
}
