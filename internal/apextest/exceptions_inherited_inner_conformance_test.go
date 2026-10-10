package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// All matrix routes reject compilation natively. The shared once-per-API runner
// links only the captured named positives and verified named setup. Rejection
// checks remain at the call site, as required by the runner's contract; rejected
// programs never execute and these observations make no runtime-success claim.
func TestExceptionsInheritedInnerOrgConformance(t *testing.T) {
	type compilation struct {
		Sources           map[string]string
		Program, Expected string
	}
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		Declarations      map[string]map[string]string
		CaptureBoundaries []struct{ ID, Boundary, Reason, Status string }
		Cases             []struct {
			ID                                                                             string
			NamedCompilations, IsTestCompilations, AnonymousDeclarations, AnonymousRuntime map[string]compilation
		}
	}
	raw, err := os.ReadFile("testdata/conformance/exceptions_inherited_inner.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 9 || len(data.CaptureBoundaries) != 9 || strings.Join(data.APIVersions, ",") != "62.0,64.0,67.0" {
		t.Fatalf("oracle shape: cases=%d boundaries=%d APIs=%v", len(data.Cases), len(data.CaptureBoundaries), data.APIVersions)
	}
	var rowIDs []string
	for _, row := range data.Cases {
		rowIDs = append(rowIDs, row.ID)
	}
	sort.Strings(rowIDs)
	if strings.Join(rowIDs, ",") != "C001,C002,C003,C007,C008,C009,C010,C011,C012" {
		t.Fatalf("captured row IDs: %v", rowIDs)
	}
	var boundaryIDs []string
	for _, row := range data.CaptureBoundaries {
		if row.ID == "" || row.Boundary != "unprovisioned-package-namespace" || row.Reason == "" || row.Status != "unobserved" {
			t.Fatalf("invalid capture boundary: %#v", row)
		}
		boundaryIDs = append(boundaryIDs, row.ID)
		t.Logf("%s unobserved boundary %s: %s", row.ID, row.Boundary, row.Reason)
	}
	sort.Strings(boundaryIDs)
	if strings.Join(boundaryIDs, ",") != "C004,C005,C006,C013,C014,C015,C016,C017,C018" {
		t.Fatalf("unobserved boundary IDs: %v", boundaryIDs)
	}
	build := func(t *testing.T, api string, sources map[string]string) typesys.Index {
		t.Helper()
		root := t.TempDir()
		names := make([]string, 0, len(sources))
		for name := range sources {
			names = append(names, name)
		}
		sort.Strings(names)
		paths := make([]string, 0, len(names))
		for _, name := range names {
			path := filepath.Join(root, name+".cls")
			writeFile(t, path, sources[name])
			paths = append(paths, path)
		}
		return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
	}
	// The standard anonymous helper remains a transient declaration. Preserve
	// every declaration's original lines and blank only its body in the request.
	declarations := func(t *testing.T, source string) (map[string]string, string) {
		t.Helper()
		parsed := apexast.NewParser().ParseSource("ExceptionMatrixAnonymous.cls", source)
		out := map[string]string{}
		masked := []byte(source)
		for _, decl := range parsed.Declarations {
			if decl.Kind != apexast.DeclarationClass {
				continue
			}
			start, end := decl.Range.Start.Offset, decl.Range.End.Offset
			if start < 0 || end <= start || end > len(source) {
				t.Fatalf("invalid captured declaration range: %s", decl.Name)
			}
			out[decl.Name] = strings.Repeat("\n", decl.Range.Start.Line-1) + source[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		return out, string(masked)
	}
	observation := func(diagnostics []diagnostic.Diagnostic, withLine, all bool) string {
		var messages []string
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			if withLine {
				line := 0
				if d.Range != nil {
					line = d.Range.Start.Line
				}
				if d.NativeLine != nil {
					line = *d.NativeLine
				}
				if line > 0 {
					message = fmt.Sprintf("line %d: %s", line, message)
				}
			}
			messages = append(messages, message)
			if !all {
				break
			}
		}
		if len(messages) == 0 {
			return "compiled"
		}
		return "COMPILE_ERROR\t" + strings.Join(messages, "\n")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "exceptions", api)
			loaded := data.Declarations[api]
			if len(loaded) != 8 {
				t.Fatalf("captured named setup count: %d", len(loaded))
			}
			index := build(t, api, loaded)
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("captured named setup rejected: %#v", analysis.Diagnostics)
			}
			// Compile/link the natively admitted class set once, rather than calling
			// Run or rebuilding a VM for each compiler row. Include the independently
			// verified setup used by the native anonymous predicate controls.
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for name := range loaded {
				if _, ok := runner.sources[conformanceSourceName(name, "")]; !ok {
					t.Fatalf("captured source not linked by API runner: %s", name)
				}
			}
			runtimeCount := 0
			for _, row := range data.Cases {
				if _, captured := row.AnonymousRuntime[api]; captured {
					runtimeCount++
				}
				t.Run(row.ID, func(t *testing.T) {
					named := func(t *testing.T, captured compilation, all bool) {
						t.Helper()
						if len(captured.Sources) == 0 || captured.Expected == "" {
							t.Fatal("missing native named compilation")
						}
						analysis := sema.Analyze(build(t, api, captured.Sources))
						observed := observation(analysis.Diagnostics, false, all)
						if observed != captured.Expected {
							t.Fatalf("expected <%s> actual <%s>: %#v", captured.Expected, observed, analysis.Diagnostics)
						}
					}
					namedKind := "exact"
					if row.NamedCompilations[api].Expected == "compiled" {
						namedKind = "category"
					}
					counts.run(t, "named", "named", namedKind, func(t *testing.T) { named(t, row.NamedCompilations[api], true) })
					counts.run(t, "isTest", "@IsTest", "exact", func(t *testing.T) {
						captured, ok := row.IsTestCompilations[api]
						if !ok || !strings.HasPrefix(captured.Expected, "COMPILE_ERROR\t") {
							t.Fatal("missing native IsTest rejection")
						}
						named(t, captured, false)
					})
					anonymous := func(t *testing.T, captured compilation) {
						t.Helper()
						if captured.Program == "" || !strings.HasPrefix(captured.Expected, "COMPILE_ERROR\t") {
							t.Fatal("missing native anonymous rejection")
						}
						transient, body := declarations(t, captured.Program)
						sources := make(map[string]string, len(loaded)+len(transient))
						for name, source := range loaded {
							sources[name] = source
						}
						for name, source := range transient {
							sources[name] = source
						}
						combined := build(t, api, sources)
						// Context markers apply only to declarations submitted anonymously;
						// loaded named helpers retain their captured lexical nesting.
						context := sema.WithAnonymousDeclarationContext(combined)
						transientIndex := typesys.Index{}
						for i, typ := range context.Types {
							if _, ok := transient[strings.TrimSuffix(filepath.Base(typ.File), ".cls")]; ok {
								transientIndex.Types = append(transientIndex.Types, typ)
							} else {
								context.Types[i] = combined.Types[i]
							}
						}
						analysis := sema.AnalyzeAnonymousDeclarationsInContext(context, transientIndex)
						observed := observation(analysis.Diagnostics, true, false)
						if observed == "compiled" {
							analysis = sema.AnalyzeAnonymous(context, body, api)
							observed = observation(analysis.Diagnostics, true, false)
						}
						if observed != captured.Expected {
							t.Fatalf("expected <%s> actual <%s>: %#v", captured.Expected, observed, analysis.Diagnostics)
						}
					}
					counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) { anonymous(t, row.AnonymousDeclarations[api]) })
					if captured, ok := row.AnonymousRuntime[api]; ok {
						for name, source := range captured.Sources {
							if loaded[name] != source {
								t.Fatalf("verified runtime setup source missing: %s", name)
							}
						}
						counts.run(t, "anonymousPredicate", "anonymous", "exact", func(t *testing.T) { anonymous(t, captured) })
					}
				})
			}
			if runtimeCount != 3 {
				t.Fatalf("anonymous predicate controls: %d", runtimeCount)
			}
		})
	}
}
