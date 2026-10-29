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

// Replay the captured constructor rejections and boundary twins through both
// semantic entry points. The project conformance test replays these same sources.
func TestStandardFieldPolicyNativeAnonymousAndNamedConstructorRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/standard_describe_profiles/P0", "api"+api, "granted/slices.json"))
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			ID, API, Route, ClassName, Source, Message string
			Compiled                                   bool
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Route == "test-runner" {
				continue
			}
			t.Run(row.API+"/"+row.Route+"/"+row.ID, func(t *testing.T) {
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), []byte(`{"edition":"Developer"}`), 0600); err != nil {
					t.Fatal(err)
				}

				if row.Route == "named" {
					path := filepath.Join(root, row.ClassName+".cls")
					if err := os.WriteFile(path, []byte(row.Source), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.SourceAPIVersion = row.API
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, s)
				result := Analyze(index)
				if row.Route == "anonymous" {
					result = AnalyzeAnonymous(index, row.Source, row.API)
				}
				if got := !index.HasErrors() && !result.HasErrors(); got != row.Compiled {
					t.Fatalf("native compiled=%t (%s); Glade=%t: %v %v", row.Compiled, row.Message, got, index.Diagnostics, result.Diagnostics)
				}
			})
		}
	}
}
