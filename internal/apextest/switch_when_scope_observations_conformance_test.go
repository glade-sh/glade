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

func TestSwitchWhenScopeObservationsConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct {
			Observer, Anonymous, AnonymousSHA256            string
			Classes, SourceSHA256, Metadata, MetadataSHA256 map[string]string
		}
		Cells []struct {
			API, Context, TransportAPI, RawAnswerSHA256, RawResponseSHA256 string
			Answers                                                        map[string]int
		}
		Provenance struct{ OrgIdSHA256, DeploymentID, DeploymentTransportAPI string }
		Uncaptured []struct{ Scope, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/switch_when_scope_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	hash := func(source string) string { sum := sha256.Sum256([]byte(source)); return hex.EncodeToString(sum[:]) }
	if len(data.Cells) != 4 || len(data.Fixtures) != 2 || len(data.Provenance.OrgIdSHA256) != 64 || data.Provenance.DeploymentID == "" || data.Provenance.DeploymentTransportAPI != "67.0" || len(data.Uncaptured) != 2 {
		t.Fatal("incomplete owned switch scope observations")
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
		if seen[key] || (cell.API != "49.0" && cell.API != "67.0") || (cell.Context != "anonymous" && cell.Context != "istest") || len(cell.Answers) != 2 || len(cell.RawAnswerSHA256) != 64 || len(cell.RawResponseSHA256) != 64 {
			t.Fatal("invalid or repeated native scope", key)
		}
		if (cell.Context == "anonymous" && cell.TransportAPI != cell.API) || (cell.Context == "istest" && cell.TransportAPI != "67.0") {
			t.Fatal("changed native transport scope", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			fixture := data.Fixtures[cell.API]
			if fixture.Observer == "" || len(fixture.Classes) != 2 || hash(fixture.Anonymous) != fixture.AnonymousSHA256 {
				t.Fatal("missing captured fixture")
			}
			for name, source := range fixture.Classes {
				if hash(source) != fixture.SourceSHA256[name] || hash(fixture.Metadata[name]) != fixture.MetadataSHA256[name] || !strings.Contains(fixture.Metadata[name], "<apiVersion>"+cell.API+"</apiVersion>") {
					t.Fatal("changed source or source API", name)
				}
			}
			index := nativeBoundaryIndex(t, cell.API, fixture.Classes, nil)
			var payload string
			if cell.Context == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, fixture.Anonymous, cell.API)
				program, err := vm.CompileAnonymousWithOptions(fixture.Anonymous, vm.CompileOptions{APIVersion: cell.API})
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
					if strings.HasPrefix(line, "SWITCH_SCOPE_RESULT|") {
						payload = strings.TrimPrefix(line, "SWITCH_SCOPE_RESULT|")
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{fixture.Observer}, NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatal("missing captured named observation")
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatal("missing named observation transport", problem)
				}
				_, payload, _ = strings.Cut(problem.Message, "SWITCH_SCOPE_RESULT|")
			}
			var got map[string]int
			if json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, cell.Answers) {
				t.Fatalf("native=%v local=%q", cell.Answers, payload)
			}
		})
	}
}
