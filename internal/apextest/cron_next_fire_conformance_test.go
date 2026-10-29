package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type cronNextFireCell struct {
	API, Route, SourceCapture, MarkerSHA256 string
	Native                                  map[string]any
	CarryReason, CarryOwner                 string
	CarriedFields                           []string
}

type cronNextFireRow struct {
	ID, SourceRow, Expression, Method, Anonymous, Named string
	SourceSHA256                                        map[string]string
	Cells                                               []cronNextFireCell
}

// Each cell replays its native clock and running-user zone. This compares the
// captured values literally, including both sides of stopTest and the pre-start
// registration twin. Anonymous cleanup completion is explicitly carried; its
// cron snapshot still runs. This does not infer DST or additional cron forms.
func TestCronNextFireNativeConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes                []string
		ShapeCount, NativeObservationCount int
		Helpers, HelperSHA256              map[string]string
		TestClass                          string
		Rows                               []cronNextFireRow
	}
	raw, err := os.ReadFile("testdata/conformance/cron_next_fire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ShapeCount != 16 || len(data.Rows) != 16 || data.NativeObservationCount != 96 || len(data.Helpers) != 2 {
		t.Fatal("incomplete native cron next-fire matrix")
	}
	checkSource := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if source == "" || fmt.Sprintf("%x", sum) != want {
			t.Fatal("changed owned cron observation source")
		}
	}
	for name, source := range data.Helpers {
		checkSource(source, data.HelperSHA256[name])
	}
	seenRows := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seenRows[row.ID] || row.SourceRow == "" || row.Expression == "" || len(row.Cells) != 6 {
			t.Fatalf("invalid native row: %s", row.ID)
		}
		seenRows[row.ID] = true
		checkSource(row.Anonymous, row.SourceSHA256["anonymous"])
		checkSource(row.Named, row.SourceSHA256["named"])
		if !strings.Contains(data.TestClass, "static void "+row.Method+"(){"+row.Named+"}") {
			t.Fatalf("named source differs from exported body: %s", row.ID)
		}
		seenCells := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			hash, hashErr := hex.DecodeString(cell.MarkerSHA256)
			if seenCells[key] || len(hash) != sha256.Size || hashErr != nil || len(cell.Native) == 0 || (cell.SourceCapture != "primary" && cell.SourceCapture != "recovery") {
				t.Fatalf("invalid/repeated native cell: %s/%s", row.ID, key)
			}
			seenCells[key] = true
			if cell.CarryReason != "" && cell.CarryOwner == "" {
				t.Fatal("a carry requires an owner and its retained native value")
			}
			if cell.CarryReason != "" && (cell.Route != "anonymous" || cell.Native["complete"] != true || strings.Join(cell.CarriedFields, ",") != "complete") {
				t.Fatal("only anonymous cleanup completion is carried")
			}
		}
		for _, api := range data.APIVersions {
			for _, route := range data.Routes {
				if !seenCells[api+"/"+route] {
					t.Fatalf("missing native cell: %s/%s/%s", row.ID, api, route)
				}
			}
		}
	}
	// The raw read advances the clock by one unit; +1 samples the post-read
	// clock so the first public now read lands on the captured instant.
	clockProgram, err := vm.CompileAnonymous("Long sampleClock=System.currentTimeMillis()+1;")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "CronNextFire", api)
			classes := map[string]string{"CronNextFireObservation": data.TestClass}
			for name, source := range data.Helpers {
				classes[name] = source
			}
			index := nativeBoundaryIndex(t, api, classes, nil)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("invalid owned cron observation declarations")
			}
			cases := Discover(index, Options{SelectedClasses: []string{"CronNextFireObservation"}})
			if len(cases) != len(data.Rows) {
				t.Fatalf("named discovery: %d rows", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					kind := "exact"
					if cell.CarryReason != "" {
						kind = "partial"
					}
					route := cell.Route
					if route == "istest" {
						route = "isTest"
					}
					counts.RunKind(t, route, row.ID, row.ID+"/"+cell.Route, kind, func(t *testing.T) {
						if cell.CarryReason != "" {
							native, _ := json.Marshal(cell.Native)
							t.Logf("carried to %s: %s; native=%s", cell.CarryOwner, cell.CarryReason, native)
						}
						machine := base.newMachine()
						zone, ok := cell.Native["timezone"].(string)
						if !ok || zone == "" {
							t.Fatal("missing captured running-user zone")
						}
						machine.SetCurrentUser(storage.Record{ID: "005-cron-observer", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue(zone)}})
						if cell.Route == "istest" {
							machine.EnableTestContext()
						}
						clock, err := machine.Execute(clockProgram)
						if err != nil {
							t.Fatal(err)
						}
						nativeNow, err := time.Parse(time.RFC3339Nano, fmt.Sprint(cell.Native["now"]))
						if err != nil {
							t.Fatal(err)
						}
						machine.AdvanceDeterministicTime(nativeNow.Sub(time.UnixMilli(clock.Vars["sampleClock"].Int)))
						var result vm.Result
						if cell.Route == "anonymous" {
							analysis := sema.AnalyzeAnonymous(index, row.Anonymous, api)
							program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
							if analysis.HasErrors() || compileErr != nil {
								t.Fatalf("native executes: diagnostics=%v compile=%v", analysis.Diagnostics, compileErr)
							}
							result, err = machine.Execute(program)
						} else {
							for _, test := range cases {
								if test.MethodName != row.Method {
									continue
								}
								key := testCaseKey(test)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
								}
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								result, err = machine.ExecuteInClass(programs[key], test.ClassName)
							}
						}
						if err != nil {
							t.Fatalf("uncaught local failure: %v", err)
						}
						matches := 0
						for _, line := range result.Debug {
							if !strings.HasPrefix(line, "CRON_NEXT_FIRE|") {
								continue
							}
							matches++
							var observed map[string]any
							if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CRON_NEXT_FIRE|")), &observed); err != nil {
								t.Fatal(err)
							}
							want := make(map[string]any, len(cell.Native))
							for key, value := range cell.Native {
								want[key] = value
							}
							for _, field := range cell.CarriedFields {
								delete(observed, field)
								delete(want, field)
							}
							if !reflect.DeepEqual(observed, want) {
								native, _ := json.Marshal(cell.Native)
								t.Fatalf("native=%s; local=%s", native, line)
							}
						}
						if matches != 1 {
							t.Fatalf("expected one cron observation, got %d", matches)
						}
					})
				}
			}
		})
	}
}
