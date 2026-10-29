package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay exact native anonymous, named and @IsTest sources twice per index.
func TestStandardConstructorCompleteNativeColdWarmRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/standard_constructor_complete/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Object, Case, Route, Source, Observation string
		ClassName                                     string `json:"class_name"`
		Compiled                                      bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 54 {
		t.Fatalf("native matrix=%d, want 54", len(rows))
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Object+"/"+row.Case, func(t *testing.T) {
			root := t.TempDir()
			p := project.Project{Root: root, SourceAPIVersion: row.API}
			if row.Route != "anonymous" {
				file := filepath.Join(root, row.ClassName+".cls")
				writeSemaFile(t, file, row.Source)
				writeSemaFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
				p.ApexFiles = []string{file}
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			for _, phase := range []string{"cold", "warm"} {
				result := Analyze(index)
				if row.Route == "anonymous" {
					result = AnalyzeAnonymous(index, row.Source, row.API)
				}
				if got := !index.HasErrors() && !result.HasErrors(); got != row.Compiled {
					t.Errorf("%s: native compiled=%t; Glade=%t: %v %v", phase, row.Compiled, got, index.Diagnostics, result.Diagnostics)
					continue
				}
				if !row.Compiled {
					want := "Field does not exist: SyntheticAbsentField on " + row.Object
					if len(result.Diagnostics) != 1 {
						t.Errorf("%s: want %s; diagnostics=%v", phase, want, result.Diagnostics)
						continue
					}
					message := result.Diagnostics[0].NativeMessage
					if message == "" {
						message = result.Diagnostics[0].Message
					}
					if message != want {
						t.Errorf("%s: native %q; Glade %q", phase, want, message)
					}
				}
			}
		})
	}
}
