package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestMissingQualifiedCallNativeConformance(t *testing.T) {
	const dir = "testdata/missing_qualified_call"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[2] != "false" || row[3] == "" {
			t.Fatalf("invalid native compile rejection: %v", row)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, row[1]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			result := AnalyzeAnonymous(typesys.Index{}, string(source), row[0])
			if !result.HasErrors() {
				t.Fatalf("native rejected compilation: %s; Glade accepted: %v", row[3], result.Diagnostics)
			}
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[3] || strings.Contains(d.Message, row[3]) {
					return
				}
			}
			t.Fatalf("native message=%q; Glade diagnostics=%+v", row[3], result.Diagnostics)
		})
	}
}

func TestMissingQualifiedCallPreservesKnownReceivers(t *testing.T) {
	utility := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass, Name: "Utility", Namespace: "owned",
		Members: []typesys.MemberSymbol{{
			Kind: apexast.DeclarationMethod, Name: "invoke", Type: "void",
			Modifiers: []string{"public", "static"},
		}},
	}
	for _, dependency := range []bool{false, true} {
		name := "project type without namespace symbol"
		if dependency {
			name = "dependency type without namespace symbol"
		}
		t.Run(name, func(t *testing.T) {
			utility.Dependency = dependency
			index := typesys.Index{Types: []typesys.TypeSymbol{utility}}
			result := AnalyzeAnonymous(index, "owned.Utility.invoke();", "67.0")
			if result.HasErrors() {
				t.Fatalf("known qualified receiver rejected: %+v", result.Diagnostics)
			}
		})
	}
	t.Run("loaded external namespace", func(t *testing.T) {
		index := typesys.Index{Dependencies: []typesys.DependencyInfo{{Namespace: "owned", Status: "loaded"}}}
		result := AnalyzeAnonymous(index, "owned.Utility.invoke();", "67.0")
		if result.HasErrors() {
			t.Fatalf("loaded external namespace policy changed: %+v", result.Diagnostics)
		}
	})
}
