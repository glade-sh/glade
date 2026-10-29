package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestQualifiedReceiverWideningAnonymousNativeConformance(t *testing.T) {
	const dir = "testdata/qualified_receiver_widening"
	cases := readQualifiedReceiverTSV(t, filepath.Join(dir, "case_table.tsv"), 3)
	if len(cases) != 57 {
		t.Fatalf("case table rows=%d, want header plus 56 cases", len(cases))
	}
	nativeRows := readQualifiedReceiverTSV(t, filepath.Join(dir, "org.tsv"), 4)
	if len(nativeRows) != len(cases) {
		t.Fatalf("native rows=%d, case table rows=%d", len(nativeRows), len(cases))
	}
	deferred := namespaceAbsenceDeferrals(t, dir)
	for name := range namespaceAbsenceDeferrals(t, filepath.Join(dir, "partial_catalog")) {
		deferred[name] = true
	}
	native := make(map[string][]string, len(nativeRows)-1)
	for _, row := range nativeRows[1:] {
		key := row[0] + "/" + row[1]
		if _, exists := native[key]; exists {
			t.Fatalf("duplicate native row %q", key)
		}
		if row[2] != "true" && row[2] != "false" {
			t.Fatalf("invalid native compile result for %s: %q", key, row[2])
		}
		if (row[2] == "false") != (row[3] != "-") {
			t.Fatalf("native result/message mismatch for %s: %v", key, row)
		}
		native[key] = row
	}

	index := qualifiedReceiverWideningIndex(t, dir)
	dependencyIndex := index
	dependencyIndex.Dependencies = []typesys.DependencyInfo{{Namespace: "Dep", SourceRoot: "owned-dependency", Version: "67.0", Status: "loaded"}}
	dependencyIndex.Types = append(dependencyIndex.Types, typesys.TypeSymbol{
		Kind: apexast.DeclarationClass, Name: "Known", Namespace: "Dep", File: "owned-dependency/Known.cls",
		Version: "67.0", EffectiveAPIVersion: "67.0", Dependency: true,
	})

	seen := make(map[string]bool, len(cases)-1)
	for _, row := range cases[1:] {
		key := row[0] + "/" + row[1]
		source, err := os.ReadFile(filepath.Join(dir, row[2]))
		if err != nil {
			t.Fatal(err)
		}
		nativeRow, ok := native[key]
		if !ok {
			t.Fatalf("case %q has no native row", key)
		}
		seen[key] = true
		caseIndex := index
		if row[1] == "dependency_bogus_member" {
			caseIndex = dependencyIndex
		}
		result := AnalyzeAnonymous(caseIndex, string(source), row[0])
		t.Run(key, func(t *testing.T) {
			wantCompile := nativeRow[2] == "true" || deferred[row[1]]
			if wantCompile == result.HasErrors() {
				t.Fatalf("native compiled=%t with message %q; Glade diagnostics=%+v", wantCompile, nativeRow[3], result.Diagnostics)
			}
		})
	}
	if len(seen) != len(native) {
		t.Fatalf("native row coverage=%d, case table coverage=%d", len(native), len(seen))
	}
}

func qualifiedReceiverWideningIndex(t *testing.T, dir string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	rollupSource, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes/Rollup.cls"))
	if err != nil {
		t.Fatal(err)
	}
	rollupClass := filepath.Join(root, "Rollup.cls")
	writeSemaFile(t, rollupClass, string(rollupSource))
	gatewaySource, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes/GatewayService.cls"))
	if err != nil {
		t.Fatal(err)
	}
	gatewayClass := filepath.Join(root, "GatewayService.cls")
	writeSemaFile(t, gatewayClass, string(gatewaySource))
	index := typesys.Build(project.Project{
		Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{rollupClass, gatewayClass},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Contact"},
		{Name: "Thing__c"},
		{Name: "Settings__c", CustomSettingsType: "List", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}},
	}})
	if len(index.Diagnostics) != 0 {
		t.Fatalf("synthetic receiver-shape index diagnostics: %#v", index.Diagnostics)
	}
	index.VisualforcePagesKnown = true
	index.VisualforcePageNames = []string{"ownedlanding"}
	return index
}

func readQualifiedReceiverTSV(t *testing.T, path string, fields int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if len(row) != fields {
			t.Fatalf("%s row %d has %d fields, want %d", path, i+1, len(row), fields)
		}
	}
	return rows
}
