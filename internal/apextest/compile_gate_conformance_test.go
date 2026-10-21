package apextest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

type compileGateNative struct {
	Status  string `json:"status"`
	Compile []struct {
		Name    string `json:"name"`
		Line    int    `json:"line"`
		Problem string `json:"problem"`
	} `json:"compile"`
	TestFailures []struct {
		Class   string `json:"cls"`
		Method  string `json:"method"`
		Message string `json:"message"`
	} `json:"testFailures"`
	TestPasses []struct {
		Class  string `json:"cls"`
		Method string `json:"method"`
	} `json:"testPasses"`
}

type compileGateRow struct {
	ID          string                       `json:"id"`
	Classes     map[string]string            `json:"classes"`
	ClassAPIs   map[string]string            `json:"classApis"`
	Tests       []string                     `json:"tests"`
	Native      map[string]compileGateNative `json:"native"`
	RuntimeTwin string                       `json:"runtimeTwin"`
	// RuntimeTag is the row tag in the sources when it is not the row ID.
	RuntimeTag string `json:"runtimeTag"`
	// Routes lists the native routes the row applies to. Every gate row is
	// deploy-only (see routeApplicability in the testdata).
	Routes []string `json:"routes"`
}

// compileGateCarry names a native fact that glade does not match yet for one
// class in some rows. The conformance test asserts every other fact of those
// rows and logs the carried one with its owner; it never asserts the local
// value. A wording carry is a root problem whose text differs; an entry carry
// is a class excluded with a different root and message, and, only where
// lineWaived says so, a different line.
type compileGateCarry struct {
	Check     string   `json:"check"`
	Kind      string   `json:"kind"`
	RowIDs    []string `json:"rowIds"`
	ClassName string   `json:"className"`
	Native    string   `json:"native"`
	Reason    string   `json:"reason"`
	// LineWaived marks an entry carry whose line differs from native too; any
	// other carry still asserts the line.
	LineWaived bool `json:"lineWaived"`
}

// compileGateCarryOwners maps a carried check to the work that owns it.
var compileGateCarryOwners = map[string]string{
	"GLADESEMA018":         "uses native wording for illegal assignment",
	"GLADESEMA008":         "unknown type static call reports Variable does not exist",
	"inherited-inner-type": "compile gate report a dependents own invalid inherited inner type",
	"duplicate-method":     "compile gate native duplicate method message",
	"GLADESEMA_VERSION":    "compile gate native Invalid api version message",
	"field-initializer":    "sema field initializer type mismatch is not checked",
}

// GATE rows (roadmap doc-7 D40): glade excludes exactly the classes a
// check-only deploy flags, in the same order, with the same lines, problems
// and dependent nesting, and runs every other test with the native runtime
// value. A class metadata error compiles nothing, so it keeps the whole
// project from running.
func TestCompileGateOrgConformance(t *testing.T) {
	var data struct {
		APIVersions        []string           `json:"apiVersions"`
		RouteApplicability map[string]string  `json:"routeApplicability"`
		Carried            []compileGateCarry `json:"carried"`
		Rows               []compileGateRow   `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/compile_gate.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Rows) != 60 {
		t.Fatalf("native matrix shape: APIs=%v rows=%d", data.APIVersions, len(data.Rows))
	}
	for _, carry := range data.Carried {
		if _, ok := compileGateCarryOwners[carry.Check]; !ok || (carry.Kind != "wording" && carry.Kind != "entry") || (carry.LineWaived && carry.Kind != "entry") {
			t.Fatalf("carry %#v has no owner or kind", carry)
		}
		if carry.Check == "GLADESEMA018" && carry.Kind == "wording" && carry.Native == "Illegal assignment from String to Integer" {
			t.Fatalf("String-to-Integer root and dependent wording must be exact, not carried: %#v", carry)
		}
	}
	// The gate is a deploy and compile concept: anonymous Apex cannot declare
	// top-level classes or leave one invalid, so every row is deploy-only and
	// says so with its reason recorded once.
	if strings.TrimSpace(data.RouteApplicability["deploy"]) == "" || strings.TrimSpace(data.RouteApplicability["anonymous"]) == "" {
		t.Fatalf("routeApplicability = %#v, want the deploy route and the reason anonymous does not apply", data.RouteApplicability)
	}
	for _, row := range data.Rows {
		if !slices.Equal(row.Routes, []string{"deploy"}) {
			t.Fatalf("row %s routes = %v, want [deploy]", row.ID, row.Routes)
		}
	}
	rows := make(map[string]compileGateRow, len(data.Rows))
	for _, row := range data.Rows {
		rows[row.ID] = row
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					native, ok := row.Native[api]
					if !ok || len(row.Classes) == 0 {
						t.Fatalf("row %s has no native answer at %s", row.ID, api)
					}
					carried := func(kind, className, problem string) (compileGateCarry, bool) {
						for _, carry := range data.Carried {
							if carry.Kind == kind && slices.Contains(carry.RowIDs, row.ID) && carry.ClassName == className && carry.Native == problem {
								return carry, true
							}
						}
						return compileGateCarry{}, false
					}
					logCarry := func(carry compileGateCarry, local string) {
						t.Logf("%s %s %s carried to %s (%s): native %q local %q", row.ID, carry.ClassName, carry.Kind, compileGateCarryOwners[carry.Check], carry.Check, carry.Native, local)
					}
					// checkMessage asserts a native problem of className: exactly, or,
					// when the innermost root problem is a carried wording gap, the
					// dependent chain before it. Retired String-to-Integer carries
					// compare the full native root/dependent message literally.
					checkMessage := func(className, native, local string) {
						root, rootProblem := className, native
						if chain := compileGateNativeChain(native); len(chain) > 0 {
							root = chain[len(chain)-1]
							for _, name := range chain {
								rootProblem = strings.TrimPrefix(rootProblem, compileGateDependentPrefix+name+" : ")
							}
						}
						carry, ok := carried("wording", root, rootProblem)
						if !ok {
							if local != native {
								t.Errorf("%s message\nnative %q\nlocal  %q", className, native, local)
							}
							return
						}
						prefix := strings.TrimSuffix(native, rootProblem)
						if !strings.HasPrefix(local, prefix) || local == native {
							t.Errorf("%s message %q, want native chain %q and the carried root text (stale carry if equal)", className, local, prefix)
						}
						logCarry(carry, strings.TrimPrefix(local, prefix))
					}
					run := runCompileGateRow(t, api, row.Classes, row.ClassAPIs)
					var order []string
					problems := make(map[string][]testreport.CompileProblem)
					metadata := false
					for _, failure := range native.Compile {
						if strings.Contains(failure.Name, "/") || strings.HasSuffix(failure.Name, "-meta.xml") {
							metadata = true
						}
						if _, ok := problems[failure.Name]; !ok {
							order = append(order, failure.Name)
						}
						problems[failure.Name] = append(problems[failure.Name], testreport.CompileProblem{Line: failure.Line, Message: failure.Problem})
					}
					if wantOK := native.Status != "Failed" && len(native.Compile) == 0 && len(native.TestFailures) == 0 && row.RuntimeTwin == ""; run.Summary().OK() != wantOK {
						t.Errorf("summary %#v disagrees with the native outcome %q", run.Summary(), native.Status)
					}
					if metadata {
						// The deploy reports only the metadata file and compiles no
						// class: every selected test is a compile error with that problem.
						if len(run.ExcludedClasses) != 0 || run.Summary().Total == 0 {
							t.Fatalf("metadata error: excluded %#v suites %#v", run.ExcludedClasses, run.Suites)
						}
						for _, suite := range run.Suites {
							for _, testCase := range suite.Cases {
								if testCase.Status != testreport.StatusCompileError || testCase.Problem == nil {
									t.Errorf("%s.%s = %#v, want compile error", suite.Name, testCase.MethodName, testCase)
									continue
								}
								for _, name := range order {
									native := problems[name][0].Message
									if carry, ok := carried("wording", name, native); ok {
										logCarry(carry, testCase.Problem.Message)
									} else if !strings.Contains(testCase.Problem.Message, native) {
										t.Errorf("%s.%s problem %q, want %q", suite.Name, testCase.MethodName, testCase.Problem.Message, native)
									}
								}
							}
						}
						return
					}
					var got []string
					excluded := make(map[string]testreport.CompileExclusion)
					for _, entry := range run.ExcludedClasses {
						got = append(got, entry.ClassName)
						excluded[entry.ClassName] = entry
					}
					if !slices.Equal(got, order) {
						t.Fatalf("excluded %v, native compile order %v\n%#v", got, order, run.ExcludedClasses)
					}
					for _, name := range order {
						entry, want := excluded[name], problems[name]
						carry, entryCarried := carried("entry", name, want[0].Message)
						if entry.Line != want[0].Line && !(entryCarried && carry.LineWaived) {
							t.Errorf("%s line %d, native %d", name, entry.Line, want[0].Line)
						}
						if entryCarried {
							// The exclusion, its order and, unless waived, its line are
							// asserted; the root and message are logged.
							// Carried differences are logged, never asserted to persist (doc-1).
							if entry.Message == want[0].Message && entry.Line == want[0].Line {
								t.Logf("carry now matches: %s (%s)", name, compileGateCarryOwners[carry.Check])
							}
							if carry.LineWaived && entry.Line == want[0].Line {
								t.Logf("carry now matches: %s line waiver (%s)", name, compileGateCarryOwners[carry.Check])
							}
							logCarry(carry, fmt.Sprintf("line %d (native %d): %s", entry.Line, want[0].Line, entry.Message))
							continue
						}
						checkMessage(name, want[0].Message, entry.Message)
						if len(want) > 1 {
							if len(entry.Problems) != len(want) {
								t.Errorf("%s problems %#v, native %#v", name, entry.Problems, want)
							}
							for i := 0; i < len(want) && i < len(entry.Problems); i++ {
								if entry.Problems[i].Line != want[i].Line {
									t.Errorf("%s problem %d line %d, native %d", name, i, entry.Problems[i].Line, want[i].Line)
								}
								checkMessage(name, want[i].Message, entry.Problems[i].Message)
							}
						} else if len(entry.Problems) != 0 {
							t.Errorf("%s problems %#v, native has one", name, entry.Problems)
						}
						chain := compileGateNativeChain(want[0].Message)
						if len(chain) == 0 {
							if entry.Root != name || entry.DependsOn != "" {
								t.Errorf("%s root entry %#v", name, entry)
							}
							continue
						}
						if entry.DependsOn != chain[0] || entry.Root != chain[len(chain)-1] {
							t.Errorf("%s dependent entry %#v, native chain %v", name, entry, chain)
						}
					}
					tag := row.RuntimeTag
					if tag == "" {
						tag = row.ID
					}
					expected := make(map[string]string)
					for _, failure := range native.TestFailures {
						expected[failure.Class+"."+failure.Method] = failure.Message
					}
					if len(native.TestPasses) != 0 {
						t.Fatalf("row %s native passes need an expectation: %#v", row.ID, native.TestPasses)
					}
					if row.RuntimeTwin != "" {
						twin := rows[row.RuntimeTwin]
						for name, source := range twin.Classes {
							if strings.ReplaceAll(source, "P|"+twin.ID+"|", "P|"+tag+"|") != row.Classes[name] {
								t.Fatalf("runtime twin %s source %s differs from %s", twin.ID, name, row.ID)
							}
						}
						for _, failure := range twin.Native[api].TestFailures {
							expected[failure.Class+"."+failure.Method] = strings.ReplaceAll(failure.Message, "P|"+twin.ID+"|", "P|"+tag+"|")
						}
					}
					for _, suite := range run.Suites {
						for _, testCase := range suite.Cases {
							name := testCase.ClassName + "." + testCase.MethodName
							if want, ok := problems[testCase.ClassName]; ok {
								if testCase.Status != testreport.StatusCompileError || testCase.Problem == nil {
									t.Errorf("%s excluded test = %#v, want compile error", name, testCase)
									continue
								}
								if entry := excluded[testCase.ClassName]; entry.DependsOn != "" && !strings.Contains(testCase.Problem.Message, entry.Message) {
									t.Errorf("%s excluded test problem %q, want the dependent message %q", name, testCase.Problem.Message, entry.Message)
								}
								if _, ok := carried("wording", testCase.ClassName, want[0].Message); !ok && compileGateNativeChain(want[0].Message) == nil && !strings.Contains(testCase.Problem.Message, want[0].Message) {
									t.Errorf("%s excluded test problem %q, want %q", name, testCase.Problem.Message, want[0].Message)
								}
								continue
							}
							want, ok := expected[name]
							if !ok {
								t.Errorf("%s ran without a native expectation: %#v", name, testCase)
								continue
							}
							delete(expected, name)
							if testCase.Status != testreport.StatusFail || testCase.Problem == nil || testCase.Problem.Type+": "+testCase.Problem.Message != want {
								t.Errorf("%s = %#v, native %q", name, testCase, want)
							}
						}
					}
					if len(expected) != 0 {
						t.Errorf("native tests did not run: %v", expected)
					}
				})
			}
		})
	}
}

func runCompileGateRow(t *testing.T, api string, classes, classAPIs map[string]string) testreport.Run {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	for name, source := range classes {
		path := filepath.Join(root, "force-app", "main", "default", "classes", name+".cls")
		classAPI := api
		if version, ok := classAPIs[name]; ok {
			classAPI = version
		}
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+classAPI+`</apiVersion><status>Active</status></ApexClass>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	return RunCasesContext(context.Background(), index, Options{
		NoDiskCache:    true,
		BuildArtifacts: &artifacts,
		SourceDigests:  artifacts.SourceDigests,
	}, Discover(index, Options{}))
}

// compileGateNativeChain returns the classes a native dependent message names,
// nearest dependency first and root last. A root problem returns nil.
func compileGateNativeChain(problem string) []string {
	var chain []string
	for strings.HasPrefix(problem, compileGateDependentPrefix) {
		name, rest, ok := strings.Cut(strings.TrimPrefix(problem, compileGateDependentPrefix), " : ")
		if !ok {
			break
		}
		chain = append(chain, name)
		problem = rest
	}
	return chain
}
