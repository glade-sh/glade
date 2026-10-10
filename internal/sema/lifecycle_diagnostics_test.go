package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// D001-D014 are API 62/67 native controls for C015/C022/C023 (overloads),
// C017/C018 (override identity), and C009/C011/C012 versus C043 (static lines).
func TestLifecycleDiagnosticSelection(t *testing.T) {
	var rows []struct {
		ID, APIVersion, Name, Source, Expected string
		Anonymous                              bool
	}
	raw, err := os.ReadFile("testdata/lifecycle_diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 28 {
		t.Fatalf("captured controls = %d, want 14 at each API", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		key := row.APIVersion + "/" + row.ID
		if seen[key] || (row.APIVersion != "62.0" && row.APIVersion != "67.0") || row.Source == "" || row.Expected == "" {
			t.Fatalf("invalid captured control: %#v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, row.Name+".cls")
			source := row.Source
			if row.Anonymous {
				// Index the captured declaration without its anonymous local.
				// Keep its original source lines for exact field diagnostics.
				parsed := apexast.NewParser().ParseSource(file, source)
				found := false
				for _, decl := range parsed.Declarations {
					if decl.Kind == apexast.DeclarationClass && decl.Name == row.Name {
						source = strings.Repeat("\n", decl.Range.Start.Line-1) + source[decl.Range.Start.Offset:decl.Range.End.Offset]
						found = true
						break
					}
				}
				if !found {
					t.Fatal("captured anonymous declaration missing")
				}
			}
			writeSemaFile(t, file, source)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.APIVersion, ApexFiles: []string{file}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("control index: %#v", index.Diagnostics)
			}
			var result Result
			if row.Anonymous {
				index = WithAnonymousDeclarationContext(index)
				result = AnalyzeAnonymousDeclarations(index)
			} else {
				result = Analyze(index)
			}
			var errors []diagnostic.Diagnostic
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error {
					errors = append(errors, d)
				}
			}
			wantErrors := 1
			if row.Anonymous {
				wantErrors = 3 // one property and two fields, each at its own range
			}
			if len(errors) != wantErrors {
				t.Fatalf("errors = %d, want %d: %#v", len(errors), wantErrors, errors)
			}
			d := errors[0]
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			observed := "COMPILE_ERROR\t" + message
			if row.Anonymous {
				if d.Range == nil {
					t.Fatal("anonymous rejection has no source range")
				}
				line := d.Range.Start.Line
				if d.NativeLine != nil {
					line = *d.NativeLine
				}
				observed = fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
			}
			if observed != row.Expected {
				t.Fatalf("expected <%s> actual <%s>", row.Expected, observed)
			}
			if !row.Anonymous {
				return
			}
			for _, typ := range index.Types {
				if typ.Name != row.Name {
					continue
				}
				for _, member := range typ.Members {
					if member.Kind != apexast.DeclarationField && member.Kind != apexast.DeclarationProperty {
						continue
					}
					found := false
					for _, d := range errors {
						if d.Range == nil || *d.Range != member.Range {
							continue
						}
						found = true
						if member.Kind == apexast.DeclarationProperty {
							if d.NativeLine == nil || *d.NativeLine != -1 {
								t.Fatalf("property %s lost its native line: %#v", member.Name, d)
							}
						} else if d.NativeLine != nil {
							t.Fatalf("field %s inherited a property line override: %#v", member.Name, d)
						}
					}
					if !found {
						t.Fatalf("member %s has no diagnostic at its own range", member.Name)
					}
				}
			}
		})
	}
}
