package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type cronTimezoneNative struct {
	Outcome          string          `json:"outcome"`
	Value            map[string]any  `json:"value"`
	ExceptionMessage string          `json:"exceptionMessage"`
	Diagnostics      json.RawMessage `json:"diagnostics"`
}

type cronTimezoneRow struct {
	ID, Group, Label, Zone, Route, Expression string
	Methods                                   []string
	Bodies                                    map[string]string
	SourceSHA256                              map[string]string
	TestClass                                 string
	ExpectedByAPI                             map[string]cronTimezoneNative
	// Carry names a fixture reason; the native values stay in ExpectedByAPI.
	Carry string
}

// Compare the printed next-fire values, keeping each measurement independent.
// Moving schedules use offsets from their own sample clock; StartTime and
// transport clocks are excluded. The GMT twins exist only on the named route.
func TestCronTimezoneNativeConformance(t *testing.T) {
	var data struct {
		APIVersions                                                                []string
		ShapeCount, NativeObservationCount, AnonymousCountPerAPI, NamedCountPerAPI int
		Helpers                                                                    map[string]string
		HelperSHA256                                                               map[string]string
		CarryReasons                                                               map[string]string
		Rows                                                                       []cronTimezoneRow
	}
	raw, err := os.ReadFile("testdata/conformance/cron_timezone/rows.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 75 || len(data.Rows) != 75 || data.NativeObservationCount != 225 || data.AnonymousCountPerAPI != 26 || data.NamedCountPerAPI != 49 || len(data.Helpers) != 2 {
		t.Fatal("incomplete native cron matrix")
	}
	for name, source := range data.Helpers {
		if source == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != data.HelperSHA256[name] {
			t.Fatalf("changed helper source: %s", name)
		}
	}
	seen := map[string]bool{}
	anonymous, named := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.Zone == "" || len(row.Methods) == 0 || len(row.Bodies) != len(row.Methods) || len(row.ExpectedByAPI) != 3 {
			t.Fatalf("invalid row: %s", row.ID)
		}
		seen[row.ID] = true
		if row.Carry != "" && data.CarryReasons[row.Carry] == "" {
			t.Fatalf("carry without a fixture reason: %s", row.ID)
		}
		for _, method := range row.Methods {
			source := row.Bodies[method]
			if source == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != row.SourceSHA256[method] {
				t.Fatalf("changed measurement source: %s/%s", row.ID, method)
			}
		}
		switch row.Route {
		case "anonymous":
			anonymous++
			if row.Zone != "America/Los_Angeles" {
				t.Fatal("anonymous zone outside native matrix")
			}
		case "istest":
			named++
			if row.TestClass == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(row.TestClass))) != row.SourceSHA256["testClass"] {
				t.Fatal("missing named source")
			}
		default:
			t.Fatal("unknown route")
		}
		var previous any
		for _, api := range data.APIVersions {
			want, ok := row.ExpectedByAPI[api]
			if !ok {
				t.Fatal("missing native API cell")
			}
			var stable any
			switch want.Outcome {
			case "runtime":
				samples := map[string]any{}
				for _, method := range row.Methods {
					samples[method] = cronTimezoneProjection(t, row, method, cronTimezoneSample(want, method))
				}
				stable = samples
			case "compile-rejected":
				if len(want.Diagnostics) == 0 || string(want.Diagnostics) == `""` || string(want.Diagnostics) == "null" {
					t.Fatal("empty native compile rejection")
				}
				// Named transport includes class names; retain diagnostics in the
				// fixture and compare rejection categories across APIs.
				stable = want.Outcome
			case "uncaught-anonymous", "uncaught-test":
				if want.ExceptionMessage == "" {
					t.Fatal("empty native uncaught exception")
				}
				stable = want.ExceptionMessage
			default:
				t.Fatalf("non-answer: %s", want.Outcome)
			}
			if previous != nil && !reflect.DeepEqual(previous, stable) {
				t.Fatalf("native API difference: %s/%s", row.ID, api)
			}
			previous = stable
		}
	}
	if anonymous != 26 || named != 49 {
		t.Fatalf("route counts: %d/%d", anonymous, named)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			clockProgram, err := vm.CompileAnonymous("Long sampleClock = System.currentTimeMillis();")
			if err != nil {
				t.Fatal(err)
			}
			counts := newConformanceCountLog(t, "CronTimezone", api)
			baseIndex := cronTimezoneIndex(t, api, data.Helpers)
			if baseIndex.HasErrors() || sema.Analyze(baseIndex).HasErrors() {
				t.Fatal("invalid cron helper sources")
			}
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				want := row.ExpectedByAPI[api]
				kind := "exact"
				if want.Outcome == "compile-rejected" {
					kind = "category"
				}
				if row.Carry != "" {
					kind = "carried"
				}
				route := row.Route
				if route == "istest" {
					route = "isTest"
				}
				counts.RunKind(t, route, row.ID, row.ID+"/"+row.Route, kind, func(t *testing.T) {
					if row.Carry != "" {
						native, _ := json.Marshal(want)
						t.Logf("carried to %s: %s; native=%s", row.Carry, data.CarryReasons[row.Carry], native)
						return
					}
					classes := make(map[string]string)
					for name, source := range data.Helpers {
						classes[name] = source
					}
					if row.Route == "istest" {
						classes["CronClockObservation"] = row.TestClass
					}
					for _, method := range row.Methods {
						t.Run(method, func(t *testing.T) {
							body := row.Bodies[method]
							if body == "" {
								t.Fatal("missing measurement source")
							}
							transient := map[string]bool{}
							if row.Route == "anonymous" {
								parsed := apexast.NewParser().ParseSource("CronClockAnonymous.cls", body)
								masked := []byte(body)
								for _, decl := range parsed.Declarations {
									if decl.Kind != apexast.DeclarationClass {
										continue
									}
									start, end := decl.Range.Start.Offset, decl.Range.End.Offset
									if start < 0 || end <= start || end > len(masked) {
										t.Fatal("invalid transient class range")
									}
									classes[decl.Name] = body[start:end]
									transient[decl.Name] = true
									for i := start; i < end; i++ {
										if masked[i] != '\n' && masked[i] != '\r' {
											masked[i] = ' '
										}
									}
								}
								body = string(masked)
							}
							index := cronTimezoneIndex(t, api, classes)
							diagnostics := append([]diagnostic.Diagnostic{}, index.Diagnostics...)
							var execErr error
							if row.Route == "anonymous" {
								// Only declarations submitted in the anonymous body are
								// transient; deployed helpers stay top-level classes.
								context := sema.WithAnonymousDeclarationContext(index)
								transientIndex := typesys.Index{}
								for i, typ := range context.Types {
									if transient[strings.TrimSuffix(filepath.Base(typ.File), ".cls")] {
										transientIndex.Types = append(transientIndex.Types, typ)
									} else {
										context.Types[i] = index.Types[i]
									}
								}
								index = context
								diagnostics = append(diagnostics, sema.AnalyzeAnonymousDeclarationsInContext(index, transientIndex).Diagnostics...)
								diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, body, api).Diagnostics...)
							} else {
								diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
							}
							program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if want.Outcome == "compile-rejected" {
								if conformanceCompilerText(diagnostics, compileErr) == "" {
									t.Fatal("native compile rejection accepted locally")
								}
								return
							}
							if text := conformanceCompilerText(diagnostics, compileErr); text != "" {
								t.Fatal(text)
							}
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							machine := runner.newMachine()
							sample := cronTimezoneSample(want, method)
							if window, ok := sample["window"].([]any); ok {
								clock, err := machine.Execute(clockProgram)
								if err != nil {
									t.Fatal(err)
								}
								shift := time.Duration(int64(window[0].(float64))-clock.Vars["sampleClock"].Int) * time.Millisecond
								if row.Route == "istest" {
									shift -= time.Second
								}
								machine.AdvanceDeterministicTime(shift)
							}
							if row.Route == "anonymous" {
								_, execErr = machine.Execute(program)
							} else {
								cases := Discover(index, Options{SelectedClasses: []string{"CronClockObservation"}, SelectedMethod: method})
								if len(cases) != 1 {
									t.Fatalf("named discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							cronTimezoneValue(t, row, method, want, execErr)
						})
					}
				})
			}
		})
	}
}

func cronTimezoneIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

func cronTimezoneSample(want cronTimezoneNative, method string) map[string]any {
	if sample, ok := want.Value[method].(map[string]any); ok {
		return sample
	}
	return want.Value
}

func cronTimezoneValue(t *testing.T, row cronTimezoneRow, method string, want cronTimezoneNative, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing captured exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if strings.HasPrefix(want.Outcome, "uncaught-") {
		// Native transport prefixes the type; compare the raw exceptionMessage
		// to ExceptionMessage, never the decorated local Error string.
		nativeType, nativeMessage, ok := strings.Cut(want.ExceptionMessage, ": ")
		if !ok || nativeMessage == "" || cronTimezoneExceptionType(runtimeErr.Type) != nativeType || message != nativeMessage {
			t.Fatalf("native <%s>; local <%s: %s>", want.ExceptionMessage, runtimeErr.Type, message)
		}
		return
	}
	if cronTimezoneExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatal(execErr)
	}
	_, payload, ok := strings.Cut(message, "CRON_RESULT|"+row.ID+"|"+method+"|")
	if !ok {
		t.Fatalf("missing result envelope: %s", message)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	expected := cronTimezoneProjection(t, row, method, cronTimezoneSample(want, method))
	actual := cronTimezoneProjection(t, row, method, got)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native <%v>; local <%v>", expected, actual)
	}
}

func cronTimezoneExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func cronTimezoneProjection(t *testing.T, row cronTimezoneRow, method string, sample map[string]any) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, key := range []string{"context", "outerContext", "effect", "triggerIdQueryMatched", "value", "exceptionType", "exceptionMessage", "step", "cron", "zone", "state", "count", "types", "fields"} {
		if value, ok := sample[key]; ok {
			out[key] = value
		}
	}
	if name, ok := out["exceptionType"].(string); ok {
		out["exceptionType"] = cronTimezoneExceptionType(name)
	}
	if message, ok := out["exceptionMessage"].(string); ok {
		const prefix = "Based on configured schedule, the given trigger 'SCHEDULED_APEX_JOB_TYPE."
		if strings.HasPrefix(message, prefix) {
			_, suffix, found := strings.Cut(message[len(prefix):], "'")
			if found {
				out["exceptionMessage"] = prefix + "[redacted-id]'" + suffix
			}
		}
	}
	if fields, ok := out["fields"].([]any); ok {
		names := make([]string, len(fields))
		for i, field := range fields {
			names[i] = strings.ToLower(field.(string))
		}
		sort.Strings(names)
		out["fields"] = names
	}
	var next []any
	if times, ok := sample["times"].([]any); ok && len(times) == 4 {
		if times[0] == nil {
			out["next"] = nil
		} else {
			next = times[0].([]any)
		}
		if row.Group == "BATCH" {
			out["startEqualsNext"] = reflect.DeepEqual(times[0], times[1])
		}
	}
	if value, ok := sample["nextLocal"]; ok {
		if value == nil {
			out["next"] = nil
		} else {
			next = value.([]any)
		}
	}
	if next != nil {
		copyNext := append([]any{}, next...)
		moving := row.Expression != "" && len(strings.Fields(row.Expression)) == 6 || row.Group == "BATCH"
		if moving {
			window, ok := sample["window"].([]any)
			if !ok || len(window) != 2 {
				t.Fatal("moving sample lacks its own clock")
			}
			origin := time.UnixMilli(int64(window[0].(float64))).UTC()
			if row.Group == "BATCH" {
				origin = origin.Truncate(time.Second)
			} else {
				origin = time.Date(origin.Year(), origin.Month(), origin.Day(), 0, 0, 0, 0, time.UTC)
			}
			zones := []string{"GMT", "GMT", "America/Los_Angeles", "Asia/Kolkata"}
			if method == "local" {
				zones = []string{"GMT", "GMT", row.Zone, row.Zone, row.Zone}
			}
			for i, value := range copyNext {
				if number, ok := value.(float64); ok {
					copyNext[i] = int64(number) - origin.UnixMilli()
					continue
				}
				text, ok := value.(string)
				if !ok {
					t.Fatal("invalid next-fire tuple")
				}
				var instant time.Time
				var err error
				if i == 0 {
					instant, err = time.Parse(time.RFC3339Nano, text)
				} else {
					location, loadErr := time.LoadLocation(zones[i])
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					layout := "2006-01-02 15:04:05"
					if method == "local" && i == 3 {
						layout = "1/2/2006, 3:04 PM"
					}
					instant, err = time.ParseInLocation(layout, text, location)
				}
				if err != nil {
					t.Fatal(err)
				}
				comparisonOrigin := origin
				if row.Group == "BATCH" && method == "local" && i == 3 {
					comparisonOrigin = origin.Truncate(time.Minute)
				}
				copyNext[i] = instant.Sub(comparisonOrigin).Milliseconds()
			}
		}
		out["next"] = copyNext
	}
	return out
}
