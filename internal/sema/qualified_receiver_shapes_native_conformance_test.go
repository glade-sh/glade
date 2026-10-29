package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestQualifiedReceiverShapesAnonymousNativeConformance(t *testing.T) {
	const dir = "testdata/qualified_receiver_shapes"
	caseTable := readQualifiedReceiverTSV(t, filepath.Join(dir, "case_table.tsv"), 4)
	if len(caseTable) != 55 {
		t.Fatalf("case table rows=%d, want header plus 54 native cases", len(caseTable))
	}
	cases := make(map[string]string, len(caseTable)-1)
	for _, row := range caseTable[1:] {
		key := row[0] + "/" + row[1]
		if _, exists := cases[key]; exists {
			t.Fatalf("duplicate case table row %q", key)
		}
		cases[key] = row[3]
		source, err := os.ReadFile(filepath.Join(dir, row[1]+".apex"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(source)) != strings.TrimSpace(row[3]) {
			t.Fatalf("%s source does not match case table", key)
		}
	}

	classPath := filepath.Join(dir, "force-app/main/default/classes/KnownType.cls")
	classSource, err := os.ReadFile(classPath)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	indexedClass := filepath.Join(root, "KnownType.cls")
	writeSemaFile(t, indexedClass, string(classSource))
	rollupSource, err := os.ReadFile(filepath.Join(dir, "force-app", "main", "default", "classes", "Rollup.cls"))
	if err != nil {
		t.Fatal(err)
	}
	rollupClass := filepath.Join(root, "Rollup.cls")
	writeSemaFile(t, rollupClass, string(rollupSource))
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{indexedClass, rollupClass}}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Contact"},
		{Name: "Settings__c", CustomSettingsType: "List", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}},
	}})
	if len(index.Diagnostics) != 0 {
		t.Fatalf("synthetic receiver-shape index diagnostics: %#v", index.Diagnostics)
	}
	index.VisualforcePagesKnown = true
	index.VisualforcePageNames = []string{"ownedlanding"}

	orgRows := readQualifiedReceiverTSV(t, filepath.Join(dir, "org.tsv"), 4)
	deferred := qualifiedReceiverUndeclaredNamespaceDeferrals(t, dir)
	if len(orgRows) != 55 {
		t.Fatalf("native rows=%d, want header plus 54 cases", len(orgRows))
	}
	seen := make(map[string]bool, len(orgRows)-1)
	for _, row := range orgRows[1:] {
		key := row[0] + "/" + row[1]
		source, exists := cases[key]
		if !exists {
			t.Fatalf("native row %q has no case table entry", key)
		}
		if seen[key] {
			t.Fatalf("duplicate native row %q", key)
		}
		seen[key] = true
		if row[2] != "true" && row[2] != "false" {
			t.Fatalf("invalid native compile result for %s: %q", key, row[2])
		}
		if (row[2] == "false") != (row[3] != "-") {
			t.Fatalf("native result/message mismatch for %s: %v", key, row)
		}

		result := AnalyzeAnonymous(index, source, row[0])
		t.Run(key, func(t *testing.T) {
			if deferred[row[1]] {
				if row[2] != "false" || result.HasErrors() {
					t.Fatalf("deferred undeclared-namespace row: native=%s, Glade diagnostics=%+v", row[2], result.Diagnostics)
				}
				return
			}
			if row[2] == "false" && !result.HasErrors() {
				t.Fatalf("native rejected compilation (%s), but Glade accepted", row[3])
			}
			if row[2] == "false" {
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.NativeMessage == row[3] {
						return
					}
				}
				t.Fatalf("native rejection message %q is absent from NativeMessage diagnostics: %+v", row[3], result.Diagnostics)
			}
			if row[2] == "true" && result.HasErrors() {
				t.Fatalf("native compiled successfully, but Glade rejected: %+v", result.Diagnostics)
			}
		})
	}
	if len(seen) != len(cases) {
		t.Fatalf("native case coverage=%d, case table coverage=%d", len(seen), len(cases))
	}
}

// deferred.tsv names native rejections of an undeclared namespace qualifier.
// The capture org lacks that namespace; offline Glade cannot see installed
// packages or the deploying org's namespace, so it defers (accepts) these
// non-local shapes. Local declaration rows reject as natively (A04 C055).
func qualifiedReceiverUndeclaredNamespaceDeferrals(t *testing.T, dir string) map[string]bool {
	t.Helper()
	rows := readQualifiedReceiverTSV(t, filepath.Join(dir, "deferred.tsv"), 2)
	deferred := make(map[string]bool, len(rows)-1)
	for _, row := range rows[1:] {
		deferred[row[0]] = true
	}
	if len(deferred) != 4 {
		t.Fatalf("deferred rows=%d, want 4", len(deferred))
	}
	return deferred
}
