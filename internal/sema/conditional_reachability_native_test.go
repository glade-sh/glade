package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestConditionalReachabilityNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyAnalysis(t, "testdata/conditional_reachability")
}

func TestConditionalReachabilityStatementNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/conditional_reachability")
	for _, row := range rows {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := cases[row[2]].Body
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			typ := typesys.TypeSymbol{Name: "GardenBodyProbe", Kind: apexast.DeclarationClass, EffectiveAPIVersion: row[0]}
			member := typesys.MemberSymbol{Name: "observe", Kind: apexast.DeclarationMethod, Type: "void"}
			model := buildSemaTypeMemberView(typesys.Index{})
			scope := newIRSemaScope(nil)
			scope.anonymous = row[1] == "anonymous"
			result := Result{Diagnostics: NewAnalyzer().checkIRStatementContracts(typ, member, program.Instructions, scope, 0, body, model)}
			if result.HasErrors() != (row[3] == "rejected") {
				t.Fatalf("native %s (%s); statement diagnostics=%v", row[3], row[4], result.Diagnostics)
			}
			if row[3] == "rejected" && (row[2] == "both_return_rejection" || row[2] == "direct_return_rejection") {
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != row[4] {
					t.Fatalf("native named/istest reachability diagnostic %q; got %v", row[4], result.Diagnostics)
				}
			}
		})
	}
}
