package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNameFieldDescribeObservationsConformance(t *testing.T) {
	var data struct {
		AnonymousSource, AnonymousSourceSHA256 string
		Fixtures                               map[string]struct {
			ClassName, Source, SourceSHA256, Metadata, MetadataSHA256 string
		}
		Cells []struct {
			API, Context, TransportAPI, RawAnswerSHA256 string
			Answers                                     map[string]bool
		}
		Provenance struct{ OrgIdSHA256, DeploymentID, DeploymentTransportAPI string }
		Uncaptured []struct{ Scope, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/name_field_describe_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	hash := func(source string) string { sum := sha256.Sum256([]byte(source)); return hex.EncodeToString(sum[:]) }
	if hash(data.AnonymousSource) != data.AnonymousSourceSHA256 || len(data.Cells) != 4 || len(data.Fixtures) != 2 || len(data.Provenance.OrgIdSHA256) != 64 || data.Provenance.DeploymentID == "" || data.Provenance.DeploymentTransportAPI != "67.0" || len(data.Uncaptured) != 1 {
		t.Fatal("incomplete owned field describe observations")
	}
	for _, carry := range data.Uncaptured {
		if carry.Scope == "" || carry.Reason == "" {
			t.Fatal("uncaptured scope requires a reason")
		}
		t.Logf("UNCAPTURED %s: %s", carry.Scope, carry.Reason)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Context
		if seen[key] || (cell.API != "62.0" && cell.API != "67.0") || (cell.Context != "anonymous" && cell.Context != "istest") || len(cell.Answers) != 5 || len(cell.RawAnswerSHA256) != 64 {
			t.Fatal("invalid or repeated native scope", key)
		}
		if (cell.Context == "anonymous" && cell.TransportAPI != cell.API) || (cell.Context == "istest" && cell.TransportAPI != "67.0") {
			t.Fatal("changed native transport scope", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			fixture := data.Fixtures[cell.API]
			if fixture.ClassName == "" || hash(fixture.Source) != fixture.SourceSHA256 || hash(fixture.Metadata) != fixture.MetadataSHA256 || !strings.Contains(fixture.Metadata, "<apiVersion>"+cell.API+"</apiVersion>") {
				t.Fatal("changed captured fixture or source API")
			}
			index := nativeBoundaryIndex(t, cell.API, map[string]string{fixture.ClassName: fixture.Source}, nil)
			var payload string
			if cell.Context == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, data.AnonymousSource, cell.API)
				program, err := vm.CompileAnonymousWithOptions(data.AnonymousSource, vm.CompileOptions{APIVersion: cell.API})
				if analysis.HasErrors() || err != nil {
					t.Fatalf("native executes: diagnostics=%v compile=%v", analysis.Diagnostics, err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "NAME_FIELD|") {
						payload = strings.TrimPrefix(line, "NAME_FIELD|")
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{fixture.ClassName}, NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatal("missing captured named observation")
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatal("missing named observation transport", problem)
				}
				_, payload, _ = strings.Cut(problem.Message, "NAME_FIELD|")
			}
			var got map[string]bool
			if json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, cell.Answers) {
				t.Fatalf("native=%v local=%q", cell.Answers, payload)
			}
		})
	}
}
