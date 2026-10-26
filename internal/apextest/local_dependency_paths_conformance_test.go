package apextest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

type localDependencyObservation struct {
	Class    string `json:"class"`
	TestsRun int    `json:"testsRun"`
	Failures int    `json:"failures"`
	Outcome  string `json:"outcome"`
}

func TestLocalDependencyPathsOrgConformance(t *testing.T) {
	const fixture = "testdata/conformance/local_dependency_paths"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			t.Fatalf("invalid native row %q", line)
		}
		rows = append(rows, fields)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("native rows = %v, want a header and two API observations", rows)
	}

	consumerRoot := filepath.Join(fixture, "fixture", "consumer")
	p, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DependencyDiagnostics) != 0 {
		t.Fatalf("dependency diagnostics = %#v", p.DependencyDiagnostics)
	}
	if len(p.ManagedPackageDependencies) != 1 {
		t.Fatalf("loaded dependencies = %#v, want one local source package", p.ManagedPackageDependencies)
	}
	dependency := p.ManagedPackageDependencies[0]
	if dependency.Status != "loaded" || dependency.Project == nil || filepath.Base(dependency.SourceRoot) != "support" {
		t.Fatalf("loaded dependency = %#v, want the configured support project", dependency)
	}

	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, schema)
	run := RunCasesContext(context.Background(), index, Options{
		NoDiskCache:    true,
		BuildArtifacts: &artifacts,
		SourceDigests:  artifacts.SourceDigests,
	}, nil)
	if summary := run.Summary(); !summary.OK() || summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("local package dependency run = %#v, want two passing tests", summary)
	}

	statuses := make(map[string]testreport.Status)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			statuses[testCase.ClassName] = testCase.Status
		}
	}
	for _, row := range rows[1:] {
		if row[1] != "isTest" {
			t.Fatalf("native route = %q, want isTest", row[1])
		}
		var native localDependencyObservation
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatalf("decode native observation for API %s: %v", row[0], err)
		}
		if native.TestsRun != 1 || native.Failures != 0 || native.Outcome != "Passed" {
			t.Fatalf("API %s native observation = %#v, want one passing test", row[0], native)
		}
		if got := statuses[native.Class]; got != testreport.StatusPass {
			t.Errorf("API %s %s status = %q, want pass", row[0], native.Class, got)
		}
	}
}
