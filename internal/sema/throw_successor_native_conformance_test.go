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

func readThrowSuccessorNativeRows(t *testing.T) (map[string]string, [][]string) {
	t.Helper()
	const dir = "testdata/throw_successor"
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
		if c.ID == "" || c.Body == "" || bodies[c.ID] != "" {
			t.Fatal("invalid source case")
		}
		bodies[c.ID] = c.Body
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+6*len(cases) || len(cases) != 8 || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic,runtime_error" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		if seen[key] || bodies[row[2]] == "" || (row[0] != "62.0" && row[0] != "67.0" && row[0] != "68.0") || (row[1] != "anonymous" && row[1] != "istest") || (row[3] != "accepted" && row[3] != "rejected") || (row[3] == "rejected" && row[4] != "Unreachable statement") {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
	}
	return bodies, rows[1:]
}

func TestThrowSuccessorAnonymousAndIsTestNativeConformance(t *testing.T) {
	bodies, rows := readThrowSuccessorNativeRows(t)
	for _, row := range rows {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			index := capturedBodyIndex(t, "testdata/throw_successor", row[0], row[1], bodies[row[2]])
			result := Analyze(index)
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(index, bodies[row[2]], row[0])
			}
			if index.HasErrors() {
				t.Fatalf("source index: %v", index.Diagnostics)
			}
			assertThrowSuccessorNativeResult(t, row, result)
		})
	}
}

func TestThrowSuccessorStatementAnonymousAndIsTestNativeRows(t *testing.T) {
	bodies, rows := readThrowSuccessorNativeRows(t)
	for _, row := range rows {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := bodies[row[2]]
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			typ := typesys.TypeSymbol{Name: "ThrowSuccessorProbe", Kind: apexast.DeclarationClass, EffectiveAPIVersion: row[0]}
			member := typesys.MemberSymbol{Name: "observe", Kind: apexast.DeclarationMethod, Type: "void"}
			scope := newIRSemaScope(nil)
			scope.anonymous = row[1] == "anonymous"
			model := buildSemaTypeMemberView(typesys.Index{})
			result := Result{Diagnostics: NewAnalyzer().checkIRStatementContracts(typ, member, program.Instructions, scope, 0, body, model)}
			assertThrowSuccessorNativeResult(t, row, result)
		})
	}
}

func assertThrowSuccessorNativeResult(t *testing.T, row []string, result Result) {
	t.Helper()
	if result.HasErrors() != (row[3] == "rejected") {
		t.Fatalf("native %s (%s); Glade diagnostics=%v", row[3], row[4], result.Diagnostics)
	}
	if row[3] == "rejected" {
		for _, d := range result.Diagnostics {
			if d.NativeMessage == row[4] || d.Message == row[4] {
				return
			}
		}
		t.Fatalf("native diagnostic %q; Glade diagnostics=%v", row[4], result.Diagnostics)
	}
}
