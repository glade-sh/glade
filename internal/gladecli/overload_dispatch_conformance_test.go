package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

type ownedOverloadDispatchCapture struct {
	Route string
	Rows  []struct {
		ownedThisDispatchSource
		ID, Name, Method, Compilation, Expected, Outcome, Message string
		Observation                                               string
	}
}

// Both routes retain independent native observations at both APIs. Anonymous
// sources reproduce the capture driver's single-row exception transport.
func ownedOverloadDispatchConformance(t *testing.T, api string, capture ownedOverloadDispatchCapture) {
	t.Helper()
	if capture.Route == "ANONYMOUS" {
		ownedOverloadDispatchAnonymousConformance(t, api, capture)
		return
	}
	if capture.Route != "@IsTest" || len(capture.Rows) != 68 {
		t.Fatal("incomplete overload_dispatch named capture/provenance")
	}
	seen := make(map[string]bool)
	for _, row := range capture.Rows {
		if seen[row.ID] || row.Name == "" || (strings.HasPrefix(row.ID, "R") && row.Method != row.ID) ||
			(strings.HasPrefix(row.ID, "C") && row.Method != "") {
			t.Fatalf("invalid named row identity: %+v", row)
		}
		seen[row.ID] = true
		t.Run(row.ID, func(t *testing.T) {
			source := ownedThisDispatchSourceText(t, row.ownedThisDispatchSource)
			root := ownedOverloadSourceProject(t, source, row.Name, api)
			var stdout, stderr bytes.Buffer
			if strings.HasPrefix(row.Compilation, "COMPILE_ERROR\t") {
				if row.Expected != "" || row.Outcome != "" || row.Message != "" {
					t.Fatal("compiler rejection has a fabricated runtime observation")
				}
				code := Run(context.Background(), []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}, &stdout, &stderr)
				if code != 1 {
					t.Fatalf("want native compiler rejection; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
				}
				var envelope struct{ Diagnostics []diagnostic.Diagnostic }
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				index, err := loadIndex(root)
				if err != nil {
					t.Fatal(err)
				}
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				analysis := sema.Analyze(index)
				wantCode := "GLADESEMA009"
				if row.ID == "C030" {
					wantCode = "GLADESEMA023"
				}
				if strings.Contains(row.Compilation, "Ambiguous method signature:") {
					wantCode = "GLADESEMA022"
				}
				got := ownedOverloadCompileObservation(t, analysis.Diagnostics, wantCode, false)
				if local := ownedOverloadCompileObservation(t, envelope.Diagnostics, wantCode, false); len(analysis.Diagnostics) == 0 || local != "COMPILE_ERROR\t"+analysis.Diagnostics[0].Message {
					t.Fatalf("CLI/analyzer diagnostic differs: %q / %+v", local, analysis.Diagnostics)
				}
				if got != row.Compilation {
					t.Fatalf("got=%q native=%q", got, row.Compilation)
				}
				return
			}
			if row.Compilation != "compiled" || row.Expected == "" || row.Outcome != "Fail" || row.Message != "System.AssertException: Assertion Failed: P|"+row.ID+"|"+row.Expected {
				t.Fatal("incomplete native runtime observation")
			}
			// Execute the byte-identical captured source, including its deliberate
			// terminal assertion, and compare the native payload exactly.
			code := Run(context.Background(), []string{"test", "--project", root, "--class", row.Name, "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("want terminal observation; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			run, err := decodeTestRunJSON(stdout.Bytes())
			if err != nil {
				t.Fatalf("%v: %s", err, stdout.String())
			}
			count := 0
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					count++
					if result.ClassName != row.Name || result.MethodName != row.Method || result.Status != testreport.StatusFail || result.Problem == nil ||
						result.Problem.Type+": "+result.Problem.Message != row.Message {
						t.Fatalf("native=%q result=%+v problem=%+v", row.Message, result, result.Problem)
					}
				}
			}
			if count != 1 {
				t.Fatalf("ran %d named methods, want 1", count)
			}
		})
	}
	for i := 1; i <= 34; i++ {
		for _, prefix := range []string{"R", "C"} {
			if id := fmt.Sprintf("%s%03d", prefix, i); !seen[id] {
				t.Fatalf("missing named row %s", id)
			}
		}
	}
}

func ownedOverloadDispatchAnonymousConformance(t *testing.T, api string, capture ownedOverloadDispatchCapture) {
	t.Helper()
	if len(capture.Rows) != 68 {
		t.Fatal("incomplete overload_dispatch anonymous capture/provenance")
	}
	seen := make(map[string]bool)
	runtimeRows, rejectionRows := 0, 0
	for _, row := range capture.Rows {
		if seen[row.ID] || row.Observation == "" || row.Name != "" || row.Method != "" ||
			row.Compilation != "" || row.Expected != "" || row.Outcome != "" || row.Message != "" {
			t.Fatalf("invalid anonymous row identity/observation: %+v", row)
		}
		seen[row.ID] = true
		rejected := strings.HasPrefix(row.Observation, "COMPILE_ERROR\t")
		if rejected {
			rejectionRows++
		} else {
			runtimeRows++
			if !strings.HasPrefix(row.ID, "R") {
				t.Fatalf("compile control has a runtime observation: %s", row.ID)
			}
		}
		t.Run(row.ID, func(t *testing.T) {
			source := ownedThisDispatchSourceText(t, row.ownedThisDispatchSource)
			root := ownedThisDispatchAnonymousProject(t, api)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
			if code != 1 || stdout.Len() != 0 {
				t.Fatalf("want native terminal observation/rejection; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			if !rejected {
				payload, err := json.Marshal([]string{"P|" + row.ID + "|" + row.Observation})
				if err != nil {
					t.Fatal(err)
				}
				want := "glade: FamilyProbeTransportException: GLADE_FAMILY_ROWS|" + string(payload) + "\n"
				if stderr.String() != want {
					t.Fatalf("anonymous runtime=%q native=%q", stderr.String(), want)
				}
				return
			}
			load := loadExecProject(root, false)
			if load.err != nil {
				t.Fatal(load.err)
			}
			if load.index.HasErrors() {
				t.Fatal(load.index.Diagnostics)
			}
			prepared, err := prepareAnonymousSourceInContext(source, load.index)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.close()
			if prepared.apiVersion != api {
				t.Fatalf("prepared API=%q, want %q", prepared.apiVersion, api)
			}
			analysis := sema.AnalyzeAnonymous(mergeAnonymousIndex(load.index, prepared.index), prepared.body, prepared.apiVersion)
			wantCode := "GLADESEMA009"
			if row.ID == "C030" {
				wantCode = "GLADESEMA023"
			}
			if strings.Contains(row.Observation, "Ambiguous method signature:") {
				wantCode = "GLADESEMA022"
			}
			got := ownedOverloadCompileObservation(t, analysis.Diagnostics, wantCode, true)
			for _, item := range analysis.Diagnostics {
				if item.Severity == diagnostic.Error && stderr.String() != "glade: "+item.Code+": "+item.Message+"\n" {
					t.Fatalf("anonymous CLI/analyzer diagnostic differs: %q / %+v", stderr.String(), item)
				}
			}
			if got != row.Observation {
				t.Fatalf("anonymous rejection=%q native=%q", got, row.Observation)
			}
		})
	}
	if runtimeRows != 31 || rejectionRows != 37 {
		t.Fatalf("anonymous rows: %d runtime + %d rejections, want 31 + 37", runtimeRows, rejectionRows)
	}
	for i := 1; i <= 34; i++ {
		for _, prefix := range []string{"R", "C"} {
			if id := fmt.Sprintf("%s%03d", prefix, i); !seen[id] {
				t.Fatalf("missing anonymous row %s", id)
			}
		}
	}
}
