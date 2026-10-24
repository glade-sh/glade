package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestThrowSuccessorScopeAnonymousAnalysisAndStatementNativeRoutes(t *testing.T) {
	const dir = "testdata/throw_successor_scope"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, c := range cases {
		bodies[c.ID] = c.Body
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(cases) != 6 || len(rows) != 13 {
		t.Fatalf("incomplete native scope matrix: %v", err)
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := bodies[row[2]]
			if body == "" || row[1] != "anonymous" {
				t.Fatal("missing anonymous source")
			}
			t.Run("analysis", func(t *testing.T) {
				assertThrowSuccessorNativeResult(t, row, AnalyzeAnonymous(typesys.Index{}, body, row[0]))
			})
			t.Run("statement", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				typ := typesys.TypeSymbol{Name: "ThrowScopeProbe", Kind: apexast.DeclarationClass, EffectiveAPIVersion: row[0]}
				member := typesys.MemberSymbol{Name: "observe", Kind: apexast.DeclarationMethod, Type: "void"}
				scope := newIRSemaScope(nil)
				scope.anonymous = true
				model := buildSemaTypeMemberView(typesys.Index{})
				result := Result{Diagnostics: NewAnalyzer().checkIRStatementContracts(typ, member, program.Instructions, scope, 0, body, model)}
				assertThrowSuccessorNativeResult(t, row, result)
			})
		})
	}
}
