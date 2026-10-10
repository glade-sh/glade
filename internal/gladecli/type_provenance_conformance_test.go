package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

type ownedProvenanceRow struct {
	ownedThisDispatchSource
	ID, Observation, Name, Method string
	Companions                    []struct {
		ownedThisDispatchSource
		Name string
	}
}

type ownedProvenanceCapture struct {
	Route string
	Rows  []ownedProvenanceRow
}

// Each capture is independent: an absent route is unobserved, not copied from
// another route. Named rows may add the captured declaring-owner companions.
func TestTypeProvenanceConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	data, err := os.ReadFile(filepath.Join("testdata", "type_provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		APIVersions []string
		Captures    map[string][]ownedProvenanceCapture
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fixture.APIVersions, ",") != "62.0,67.0" || len(fixture.Captures) != 2 {
		t.Fatal("incomplete provenance API matrix")
	}
	for _, api := range fixture.APIVersions {
		if len(fixture.Captures[api]) != 2 {
			t.Fatal("missing anonymous/@IsTest native capture")
		}
		routes := make(map[string]bool)
		for _, capture := range fixture.Captures[api] {
			if routes[capture.Route] {
				t.Fatal("duplicate capture route")
			}
			routes[capture.Route] = true
			t.Run(api+"/"+capture.Route, func(t *testing.T) {
				ownedProvenanceConformance(t, api, capture)
			})
		}
	}
}

func ownedProvenanceConformance(t *testing.T, api string, capture ownedProvenanceCapture) {
	t.Helper()
	ids := []int{2, 4, 5, 6, 7, 8, 14, 15, 16, 17, 18, 19}
	if capture.Route == "@IsTest" {
		ids = nil
		for i := 1; i <= 27; i++ {
			ids = append(ids, i)
		}
	} else if capture.Route != "ANONYMOUS" {
		t.Fatal("unknown native route")
	}
	if len(capture.Rows) != len(ids)*2 {
		t.Fatal("incomplete native route matrix")
	}
	seen := make(map[string]bool)
	runtimeRows, rejections, compileAcceptances, matched := 0, 0, 0, 0
	for _, row := range capture.Rows {
		if seen[row.ID] || row.Observation == "" || row.Observation == "MISSING" {
			t.Fatal("duplicate/unobserved native row")
		}
		seen[row.ID] = true
		if strings.HasPrefix(row.Observation, "COMPILE_ERROR\t") {
			rejections++
		} else if capture.Route == "@IsTest" && strings.HasPrefix(row.ID, "C") && row.Observation == "compiled" {
			compileAcceptances++
		} else {
			runtimeRows++
		}
		t.Run(row.ID, func(t *testing.T) {
			defer func() {
				if !t.Failed() {
					matched++
				}
			}()
			source := ownedThisDispatchSourceText(t, row.ownedThisDispatchSource)
			root := ownedThisDispatchAnonymousProject(t, api)
			if capture.Route == "@IsTest" {
				if row.Name == "" {
					t.Fatal("missing native class")
				}
				root = ownedOverloadSourceProject(t, source, row.Name, api)
				for _, companion := range row.Companions {
					if companion.Name == "" || filepath.Base(companion.Name) != companion.Name {
						t.Fatal("invalid companion class")
					}
					text := ownedThisDispatchSourceText(t, companion.ownedThisDispatchSource)
					path := filepath.Join(root, "force-app", "main", "default", "classes", companion.Name+".cls")
					writeTestFile(t, path, text)
					writeTestFile(t, path+"-meta.xml", fmt.Sprintf(`<?xml version="1.0"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				}
			} else if row.Name != "" || row.Method != "" || len(row.Companions) != 0 {
				t.Fatal("anonymous row contains named sources")
			}
			var stdout, stderr bytes.Buffer
			rejected := strings.HasPrefix(row.Observation, "COMPILE_ERROR\t")
			args := []string{"exec", "--project", root, "--json", source}
			if capture.Route == "@IsTest" {
				if rejected || row.Observation == "compiled" {
					args = []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}
				} else {
					if row.Method != row.ID {
						t.Fatal("missing captured method")
					}
					args = []string{"test", "--project", root, "--class", row.Name, "--no-cache", "--no-serve", "--no-progress", "--json"}
				}
			}
			code := Run(context.Background(), args, &stdout, &stderr)
			if capture.Route == "@IsTest" && row.Observation == "compiled" {
				// The named driver does not execute accepted C controls. Assert
				// compilation only, preserving that native observation boundary.
				if !strings.HasPrefix(row.ID, "C") || row.Method != "" || code != 0 {
					t.Fatalf("want native compile acceptance; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
				}
				var envelope struct{ Diagnostics []diagnostic.Diagnostic }
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				for _, item := range envelope.Diagnostics {
					if item.Severity == diagnostic.Error {
						t.Fatal(item)
					}
				}
				return
			}
			if code != 1 {
				t.Fatalf("want native rejection/terminal observation; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			if !rejected {
				if capture.Route == "ANONYMOUS" {
					payload, _ := json.Marshal([]string{"P|" + row.ID + "|" + row.Observation})
					want := "glade: FamilyProbeTransportException: GLADE_FAMILY_ROWS|" + string(payload) + "\n"
					if stdout.Len() != 0 || stderr.String() != want {
						t.Fatalf("runtime=%q native=%q stdout=%s", stderr.String(), want, stdout.String())
					}
				} else {
					run, err := decodeTestRunJSON(stdout.Bytes())
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							count++
							want := "System.AssertException: Assertion Failed: P|" + row.ID + "|" + row.Observation
							if result.ClassName != row.Name || result.MethodName != row.Method || result.Status != testreport.StatusFail || result.Problem == nil || result.Problem.Type+": "+result.Problem.Message != want {
								t.Fatalf("native=%q result=%+v", want, result)
							}
						}
					}
					if count != 1 {
						t.Fatalf("ran %d methods, want 1", count)
					}
				}
				return
			}
			var diagnostics []diagnostic.Diagnostic
			if capture.Route == "ANONYMOUS" {
				load := loadExecProject(root, false)
				if load.err != nil {
					t.Fatal(load.err)
				}
				prepared, err := prepareAnonymousSourceInContext(source, load.index)
				if err != nil {
					t.Fatal(err)
				}
				defer prepared.close()
				if prepared.apiVersion != api {
					t.Fatal("wrong anonymous API")
				}
				analysis := sema.AnalyzeAnonymous(mergeAnonymousIndex(load.index, prepared.index), prepared.body, api)
				diagnostics = analysis.Diagnostics
				for _, item := range diagnostics {
					if item.Severity == diagnostic.Error && stderr.String() != "glade: "+item.Code+": "+item.Message+"\n" {
						t.Fatalf("CLI/analyzer diagnostics differ: %s / %+v", stderr.String(), diagnostics)
					}
				}
				if stdout.Len() != 0 {
					t.Fatal(stdout.String())
				}
			} else {
				index, err := loadIndex(root)
				if err != nil {
					t.Fatal(err)
				}
				analysis := sema.Analyze(index)
				diagnostics = analysis.Diagnostics
				var envelope struct{ Diagnostics []diagnostic.Diagnostic }
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				var localErrors, analyzerErrors []diagnostic.Diagnostic
				for _, item := range envelope.Diagnostics {
					if item.Severity == diagnostic.Error {
						localErrors = append(localErrors, item)
					}
				}
				for _, item := range diagnostics {
					if item.Severity == diagnostic.Error {
						analyzerErrors = append(analyzerErrors, item)
					}
				}
				if len(localErrors) != len(analyzerErrors) {
					t.Fatal("CLI/analyzer error counts differ")
				}
				for i, item := range analyzerErrors {
					if localErrors[i].Code != item.Code || localErrors[i].Message != item.Message {
						t.Fatal("CLI/analyzer diagnostics differ")
					}
				}
			}
			wantCode := "GLADESEMA009"
			if strings.Contains(row.Observation, "Expression must be a list type:") {
				wantCode = "GLADESEMA019"
			} else if capture.Route == "@IsTest" && row.ID == "C023" {
				wantCode = "GLADESEMA023"
			}
			// executeAnonymous captures the first compiler diagnostic. A source
			// line can contain repeated invalid reads; each reported diagnostic
			// must match that exact native text and line, with no other errors.
			var nativeMessages []string
			for _, item := range diagnostics {
				if item.Severity != diagnostic.Error {
					continue
				}
				got := ownedOverloadCompileObservation(t, []diagnostic.Diagnostic{item}, wantCode, capture.Route == "ANONYMOUS")
				if capture.Route == "ANONYMOUS" && got != row.Observation {
					t.Fatalf("rejection=%q native=%q", got, row.Observation)
				}
				nativeMessages = append(nativeMessages, strings.TrimPrefix(got, "COMPILE_ERROR\t"))
			}
			if len(nativeMessages) == 0 {
				t.Fatal("missing native compiler rejection")
			}
			// The named driver retains every Tooling insertion error, in order.
			if capture.Route == "@IsTest" {
				got := "COMPILE_ERROR\t" + strings.Join(nativeMessages, "\n")
				if got != row.Observation {
					t.Fatalf("rejection=%q native=%q", got, row.Observation)
				}
			}
		})
	}
	for _, i := range ids {
		for _, prefix := range []string{"R", "C"} {
			if !seen[fmt.Sprintf("%s%03d", prefix, i)] {
				t.Fatal("missing native row")
			}
		}
	}
	if t.Failed() {
		return
	}
	wantRuntime, wantRejections := 10, 14
	if capture.Route == "@IsTest" {
		wantRuntime, wantRejections = 25, 29
	}
	if runtimeRows != wantRuntime || rejections != wantRejections || compileAcceptances != 0 {
		t.Fatal("native runtime/rejection coverage changed")
	}
	t.Logf("native matches: %d/%d (%d runtime, %d rejections, %d compile acceptances)", matched, len(capture.Rows), runtimeRows, rejections, compileAcceptances)
}
