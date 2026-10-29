package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardFieldPolicyOrderManagementNativeConstructorRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_field_policy_combinations"
	raw, err := os.ReadFile(filepath.Join(dir, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Profile, Route, Case, Source string
		ClassName                         string `json:"class_name"`
		SourceSHA                         string `json:"source_sha256"`
		Compiled                          bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows {
		if row.Profile != "order-management" || row.Case == "fields" {
			continue
		}
		count++
		t.Run(row.API+"/"+row.Route+"/"+row.Case, func(t *testing.T) {
			digest := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(digest[:]) != row.SourceSHA {
				t.Fatal("native source changed")
			}
			root := t.TempDir()
			definition, err := os.ReadFile(filepath.Join(dir, "order-management-definition.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if row.Route != "anonymous" {
				path := filepath.Join(root, "force-app/main/default/classes", row.ClassName+".cls")
				writeFile(t, path, row.Source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			if got := !index.HasErrors() && !analysis.HasErrors(); got != row.Compiled {
				t.Fatalf("native compiled=%t; Glade=%t: %v %v", row.Compiled, got, index.Diagnostics, analysis.Diagnostics)
			}
			if !row.Compiled {
				return
			}
			if row.Route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native constructor passed; Glade %+v", run.Summary())
				}
				return
			}
			source := row.Source
			if row.Route == "named" {
				source = row.ClassName + ".sample();"
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 12 {
		t.Fatalf("constructor route matrix=%d, want 12", count)
	}
}

func TestStandardFieldPolicyCombinationNativeEvidence(t *testing.T) {
	const dir = "testdata/conformance/standard_field_policy_combinations"
	raw, err := os.ReadFile(filepath.Join(dir, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Profile, Route, Case, Source, Observation string
		Compiled                                       bool
		SourceSHA                                      string `json:"source_sha256"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 54 || len(table) != 55 {
		t.Fatalf("native observations=%d TSV rows=%d, want 54 and header", len(rows), len(table))
	}
	seen := map[string]bool{}
	for i, row := range rows {
		key := strings.Join([]string{row.Profile, row.API, row.Route, row.Case}, "/")
		if seen[key] {
			t.Fatalf("duplicate native route %s", key)
		}
		seen[key] = true
		digest := sha256.Sum256([]byte(row.Source))
		if hex.EncodeToString(digest[:]) != row.SourceSHA {
			t.Fatalf("native source changed: %s", key)
		}
		want := []string{row.API, row.Profile, row.Route, row.Case, strconv.FormatBool(row.Compiled), row.Observation, row.SourceSHA}
		if strings.Join(table[i+1], "\t") != strings.Join(want, "\t") {
			t.Fatalf("native TSV differs: %s", key)
		}
	}
	for _, profile := range []string{"order-management", "person-address", "health-cloud", "neither", "quotes-only", "orders-only", "quotes-and-orders"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "named", "test-runner"} {
				if !seen[profile+"/"+api+"/"+route+"/fields"] {
					t.Fatalf("missing %s/%s/%s", profile, api, route)
				}
			}
		}
	}
}
