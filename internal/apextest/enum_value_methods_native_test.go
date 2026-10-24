package apextest

import (
	"github.com/glade-sh/glade/internal/testutil/enumvaluefixture"
	"strings"
	"testing"
)

func TestEnumValueMethodsNativeProjectRunnerNamedIsTestRoutes(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route == "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			enumvaluefixture.WriteProject(t, root, fixture, row)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			for _, entry := range run.ExcludedClasses {
				if entry.ClassName == row.Case.Name {
					if row.Compiled || !strings.Contains(entry.Message, row.Diagnostic) {
						t.Fatalf("org.tsv:%d excluded: %+v", row.Line, entry)
					}
					return
				}
			}
			if !row.Compiled {
				t.Fatalf("org.tsv:%d wants rejection %q; summary=%+v excluded=%+v", row.Line, row.Diagnostic, run.Summary(), run.ExcludedClasses)
			}
		})
	}
}
