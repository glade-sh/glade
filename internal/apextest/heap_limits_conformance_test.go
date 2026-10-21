package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned sync and async heap/CPU limit observations. Anonymous rows assert
// success or the uncaught exception; test rows run their native test classes
// and assert every native failure message and pass, including statics read
// after Test.stopTest. Rows run in permissive and strict limit mode unless
// they name their modes: a native heap LimitException is asserted in strict
// mode only, since permissive mode never throws one. Heap byte counts are the
// platform's own accounting and are never compared.
func TestHeapLimitsOrgConformance(t *testing.T) {
	type nativeFailure struct{ Class, Method, Message string }
	type nativeObservation struct {
		Success   bool
		Exception string
		Failures  []nativeFailure
		Passes    []struct{ Class, Method string }
	}
	type carry struct{ Owner, Reason string }
	type familyCase struct {
		ID          string
		Anonymous   string
		Classes     map[string]string
		Tests       []string
		MatchFields []string
		Modes       []vm.LimitMode
		Carried     *carry
		Native      map[string]nativeObservation
	}
	var data struct {
		APIVersions        []string `json:"apiVersions"`
		CarriedAPIVersions map[string]carry
		Cases              []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/heap_limits.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	api68Observations := 0
	for _, tc := range data.Cases {
		if _, captured := tc.Native["68.0"]; captured {
			api68Observations++
		}
	}
	if len(data.Cases) != 45 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.CarriedAPIVersions) != 0 || api68Observations != 16 {
		t.Fatalf("oracle: %d rows at %v, carried %v", len(data.Cases), data.APIVersions, data.CarriedAPIVersions)
	}
	heapBytes := regexp.MustCompile(`(Apex heap size too large: )[0-9]+`)
	// normalize keeps the text before the first observation field plus the
	// listed key=value fields, in order, and drops a heap byte count.
	normalize := func(text string, fields []string) (string, bool) {
		text = heapBytes.ReplaceAllString(text, "${1}<bytes>")
		if len(fields) == 0 {
			return text, true
		}
		wanted := map[string]bool{}
		for _, field := range fields {
			wanted[field] = true
		}
		var kept []string
		found := map[string]bool{}
		inFields := false
		for _, part := range strings.Split(text, "|") {
			key, _, isField := strings.Cut(part, "=")
			inFields = inFields || isField
			switch {
			case isField && wanted[key]:
				kept = append(kept, part)
				found[key] = true
			case !inFields:
				kept = append(kept, part)
			}
		}
		return strings.Join(kept, "|"), len(found) == len(wanted)
	}
	// expectedText is the native answer in the same form observe returns.
	expectedText := func(tc familyCase, native nativeObservation) string {
		if tc.Anonymous != "" {
			if native.Success {
				return "SUCCESS"
			}
			text, _ := normalize(native.Exception, tc.MatchFields)
			return "UNCAUGHT|" + text
		}
		var lines []string
		for _, failure := range native.Failures {
			text, _ := normalize(failure.Message, tc.MatchFields)
			lines = append(lines, strings.ToLower(failure.Class+"."+failure.Method)+"|FAIL|"+text)
		}
		for _, pass := range native.Passes {
			lines = append(lines, strings.ToLower(pass.Class+"."+pass.Method)+"|PASS")
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n")
	}
	seen := map[string]bool{}
	classOwner := map[string]string{}
	for _, tc := range data.Cases {
		if tc.ID == "" || seen[tc.ID] || (tc.Anonymous == "") == (len(tc.Classes) == 0) || len(tc.Classes) > 0 && len(tc.Tests) == 0 || tc.Carried != nil && (tc.Carried.Owner == "" || tc.Carried.Reason == "") {
			t.Fatalf("invalid row: %#v", tc)
		}
		seen[tc.ID] = true
		for name := range tc.Classes {
			if classOwner[strings.ToLower(name)] != "" {
				t.Fatalf("class %s repeated by %s and %s", name, classOwner[strings.ToLower(name)], tc.ID)
			}
			classOwner[strings.ToLower(name)] = tc.ID
		}
		for _, mode := range tc.Modes {
			if mode != vm.LimitModePermissive && mode != vm.LimitModeStrict {
				t.Fatalf("%s: unknown mode %q", tc.ID, mode)
			}
		}
		asserted := 0
		for _, api := range data.APIVersions {
			native, ok := tc.Native[api]
			if !ok {
				// HEAP3/HEAP4 were captured only at 62.0 and 67.0.
				// The 16 existing API 68.0 observations are asserted below.
				if api == "68.0" {
					continue
				}
				t.Fatalf("missing native observation %s@%s", tc.ID, api)
			}
			asserted++
			answers := len(native.Failures) + len(native.Passes)
			if native.Success {
				answers++
			}
			if native.Exception != "" {
				answers++
			}
			if answers == 0 || tc.Anonymous != "" && answers != 1 || tc.Anonymous != "" && len(native.Failures)+len(native.Passes) > 0 || tc.Anonymous == "" && (native.Success || native.Exception != "") {
				t.Fatalf("invalid native observation %s@%s", tc.ID, api)
			}
			texts := []string{native.Exception}
			for _, failure := range native.Failures {
				texts = append(texts, failure.Message)
			}
			for _, text := range texts {
				if _, complete := normalize(text, tc.MatchFields); text != "" && !complete {
					t.Fatalf("native observation %s@%s lacks fields %v: %s", tc.ID, api, tc.MatchFields, text)
				}
			}
		}
		if asserted == 0 {
			t.Fatalf("%s has no asserted API", tc.ID)
		}
	}
	problemText := func(problem *testreport.Problem) string {
		if problem == nil {
			return ""
		}
		typeName := problem.Type
		if !strings.Contains(typeName, ".") {
			typeName = "System." + typeName
		}
		return typeName + ": " + problem.Message
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			releaseRuntimeCachesAfterTest(t)
			counts := newConformanceCountLog(t, "HEAP", api)
			root := t.TempDir()
			var names []string
			sources := map[string]string{}
			for _, tc := range data.Cases {
				for name, source := range tc.Classes {
					names = append(names, name)
					sources[name] = source
				}
			}
			sort.Strings(names)
			paths := make([]string, 0, len(names))
			for _, name := range names {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, sources[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, ApexFiles: paths, SourceAPIVersion: api}, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatalf("parser: %v", index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			if carry, carried := data.CarriedAPIVersions[api]; carried {
				// The carry lapses as soon as glade accepts this source API.
				versionErrors := 0
				for _, item := range analysis.Diagnostics {
					if item.Severity == diagnostic.Error && item.Code != "GLADESEMA_VERSION" {
						t.Fatalf("carried API %s: unexpected diagnostic %v", api, item)
					}
					if item.Code == "GLADESEMA_VERSION" {
						versionErrors++
					}
				}
				if versionErrors == 0 {
					// Carried differences are logged, never asserted to persist (doc-1).
					t.Logf("carry now matches: API %s compiles (%s); assert its native rows", api, carry.Owner)
				}
				for _, tc := range data.Cases {
					if _, observed := tc.Native[api]; observed {
						t.Logf("carried %s@%s owner=%s: %s", tc.ID, api, carry.Owner, carry.Reason)
					}
				}
				return
			}
			if analysis.HasErrors() {
				t.Fatalf("semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			observe := func(t *testing.T, tc familyCase, mode vm.LimitMode) string {
				if tc.Anonymous != "" {
					if analysis := sema.AnalyzeAnonymous(index, tc.Anonymous, api); analysis.HasErrors() {
						t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(tc.Anonymous, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = anonymousRunner.execute(program, func(machine *vm.VM) { machine.SetLimitMode(mode) })
					if err == nil {
						return "SUCCESS"
					}
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) {
						return "ERROR|" + err.Error()
					}
					text, _ := normalize(problemText(&testreport.Problem{Type: runtimeErr.Type, Message: runtimeErr.ExceptionMessage()}), tc.MatchFields)
					return "UNCAUGHT|" + text
				}
				run := Run(index, Options{SelectedClasses: tc.Tests, LimitMode: mode, NoDiskCache: true, Parallelism: 1})
				var lines []string
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						key := strings.ToLower(result.ClassName + "." + result.MethodName)
						switch result.Status {
						case testreport.StatusPass:
							lines = append(lines, key+"|PASS")
						case testreport.StatusFail:
							text, _ := normalize(problemText(result.Problem), tc.MatchFields)
							lines = append(lines, key+"|FAIL|"+text)
						default:
							lines = append(lines, key+"|"+string(result.Status)+"|"+problemText(result.Problem))
						}
					}
				}
				sort.Strings(lines)
				return strings.Join(lines, "\n")
			}
			for _, tc := range data.Cases {
				native, observed := tc.Native[api]
				if !observed {
					continue
				}
				route := "isTest"
				if tc.Anonymous != "" {
					route = "anonymous"
				}
				t.Run(tc.ID, func(t *testing.T) {
					expected := expectedText(tc, native)
					if tc.Carried != nil {
						// Carried differences are logged, never asserted to persist (doc-1).
						if actual := observe(t, tc, vm.LimitModePermissive); actual == expected {
							t.Logf("carry now matches: %s@%s (%s)", tc.ID, api, tc.Carried.Owner)
						}
						t.Logf("carried %s@%s owner=%s: %s", tc.ID, api, tc.Carried.Owner, tc.Carried.Reason)
						return
					}
					modes := tc.Modes
					if len(modes) == 0 {
						modes = []vm.LimitMode{vm.LimitModePermissive, vm.LimitModeStrict}
					}
					for _, mode := range modes {
						counts.Run(t, route, tc.ID, route+"-"+string(mode), func(t *testing.T) {
							if actual := observe(t, tc, mode); actual != expected {
								t.Fatalf("native <%s>\nactual <%s>", expected, actual)
							}
						})
					}
				})
			}
		})
	}
}
