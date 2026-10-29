package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type eventListExtendedFixture struct {
	Classes      map[string]string `json:"classes"`
	Objects      map[string]string `json:"objects"`
	SourceSHA256 map[string]string `json:"sourceSHA256"`
}

type eventListExtendedCell struct {
	ID                 string            `json:"id"`
	API                string            `json:"api"`
	Route              string            `json:"route"`
	Stage              string            `json:"stage"`
	Sources            map[string]string `json:"sources"`
	SourceSHA256       map[string]string `json:"sourceSHA256"`
	SourceBinding      map[string]string `json:"sourceBinding"`
	EffectiveSources   map[string]string `json:"effectiveSources"`
	EffectiveSHA256    map[string]string `json:"effectiveSourceSHA256"`
	NativeAnswerSHA256 string            `json:"nativeAnswerSHA256"`
	CarryReason        string            `json:"carryReason"`
	NativeAnswer       struct {
		Native      json.RawMessage `json:"native"`
		Observation struct {
			Outcome    string          `json:"outcome"`
			Value      json.RawMessage `json:"value"`
			Settled    bool            `json:"settled"`
			Resolution string          `json:"resolution"`
		} `json:"observation"`
	} `json:"nativeAnswer"`
}

// Compare publication results and allocation-sensitive limits on both native
// routes. Exact hosted event identities are retained in the fixture; local
// comparisons assert their presence and equality relationships, not their values.
// The six settled platform faults are explicit carries, never local matches.
func TestEventListExtendedNativeConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                            `json:"apiVersions"`
		Routes                 []string                            `json:"routes"`
		ShapeCount             int                                 `json:"shapeCount"`
		RuntimeCellCount       int                                 `json:"runtimeCellCount"`
		PlatformFaultCellCount int                                 `json:"platformFaultCellCount"`
		FixturesByAPI          map[string]eventListExtendedFixture `json:"fixturesByAPI"`
		Cells                  []eventListExtendedCell             `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/event_list_extended_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" ||
		data.ShapeCount != 16 || data.RuntimeCellCount != 90 || data.PlatformFaultCellCount != 6 || len(data.Cells) != 96 || len(data.FixturesByAPI) != 3 {
		t.Fatal("incomplete native event-list matrix")
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("captured source bytes changed")
		}
	}
	for _, fixture := range data.FixturesByAPI {
		if len(fixture.Classes) != 1 || len(fixture.Objects) != 2 {
			t.Fatal("missing captured relay or immediate/after-commit metadata")
		}
		for name, source := range fixture.Classes {
			checkHash(source, fixture.SourceSHA256["class:"+name])
		}
		for name, source := range fixture.Objects {
			checkHash(source, fixture.SourceSHA256["object:"+name])
		}
	}
	seen := map[string]bool{}
	runtimeCells, carriedCells := 0, 0
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.ID + "/" + cell.Route
		if seen[key] || len(cell.NativeAnswerSHA256) != 64 || len(cell.Sources) != 1 || len(cell.NativeAnswer.Native) == 0 {
			t.Fatalf("duplicate or unbound native cell: %s", key)
		}
		seen[key] = true
		for name, source := range cell.Sources {
			checkHash(source, cell.SourceSHA256[name])
		}
		if len(cell.SourceBinding) != 0 {
			fixture := data.FixturesByAPI[cell.API]
			if cell.API != "68.0" || len(cell.EffectiveSources) != len(cell.Sources) || len(cell.EffectiveSHA256) != len(cell.Sources) {
				t.Fatal("unbound source reconstruction has an unexpected scope")
			}
			for placeholder, bound := range cell.SourceBinding {
				switch placeholder {
				case "SignalRelay":
					if fixture.Classes[bound] == "" {
						t.Fatal("source binding does not name the captured relay")
					}
				case "SignalPulse__e", "CommittedPulse__e":
					if fixture.Objects[bound] == "" {
						t.Fatal("source binding does not name captured event metadata")
					}
				default:
					t.Fatal("unexpected source dependency binding")
				}
			}
			for name, source := range cell.Sources {
				effective := source
				for placeholder, bound := range cell.SourceBinding {
					// Word boundaries leave already-bound identifiers untouched.
					pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(placeholder) + `\b`)
					if !pattern.MatchString(effective) {
						t.Fatal("source binding names an already-bound dependency")
					}
					effective = pattern.ReplaceAllString(effective, bound)
				}
				if effective != cell.EffectiveSources[name] {
					t.Fatal("effective source is not the captured dependency-name binding")
				}
				checkHash(effective, cell.EffectiveSHA256[name])
			}
		}
		switch cell.Stage {
		case "runtime":
			if cell.NativeAnswer.Observation.Outcome != "runtime" || len(cell.NativeAnswer.Observation.Value) == 0 {
				t.Fatal("missing native runtime payload")
			}
			runtimeCells++
		case "platform-fault-carried":
			if cell.ID != "C04-14" || cell.CarryReason == "" || cell.NativeAnswer.Observation.Outcome != "PLATFORM_FAULT" ||
				!cell.NativeAnswer.Observation.Settled || cell.NativeAnswer.Observation.Resolution != "platform-fault-reproducible" {
				t.Fatal("unsettled or unexplained platform-fault carry")
			}
			carriedCells++
		default:
			t.Fatal("unknown native event-list stage")
		}
	}
	if runtimeCells != 90 || carriedCells != 6 {
		t.Fatal("runtime/carry observations changed")
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "EventListExtended", api)
			for n := 1; n <= 16; n++ {
				id := fmt.Sprintf("C04-%02d", n)
				for _, route := range data.Routes {
					if !seen[api+"/"+id+"/"+route] {
						t.Fatalf("missing native cell: %s/%s/%s", api, id, route)
					}
				}
			}
			for _, cell := range data.Cells {
				if cell.API != api {
					continue
				}
				t.Run(cell.ID+"/"+cell.Route, func(t *testing.T) {
					if cell.Stage == "platform-fault-carried" {
						counts.TrackKind(t, cell.Route, "carried", 1)
						t.Logf("settled platform-fault carry: %s; native=%s", cell.CarryReason, cell.NativeAnswer.Native)
						return
					}
					// Exact IDs/UUIDs are retained as native evidence, while their
					// identity relations are asserted. Count this row as partial.
					counts.TrackKind(t, cell.Route, "partial", 1)
					got := eventListExtendedExecute(t, cell, data.FixturesByAPI[api])
					var want map[string]any
					if err := json.Unmarshal(cell.NativeAnswer.Observation.Value, &want); err != nil {
						t.Fatal(err)
					}
					got = eventListExtendedProjection(t, got)
					want = eventListExtendedProjection(t, want)
					if !reflect.DeepEqual(got, want) {
						actual, _ := json.Marshal(got)
						expected, _ := json.Marshal(want)
						t.Fatalf("native %s; Glade %s", expected, actual)
					}
				})
			}
		})
	}
}

func eventListExtendedExecute(t *testing.T, cell eventListExtendedCell, fixture eventListExtendedFixture) map[string]any {
	t.Helper()
	sources := cell.Sources
	if len(cell.SourceBinding) != 0 {
		// Retain the captured template separately from its verified deployed
		// dependency-name binding; line positions and measured body are unchanged.
		sources = cell.EffectiveSources
	}
	classes := make(map[string]string, len(fixture.Classes)+1)
	for name, source := range fixture.Classes {
		classes[name] = source
	}
	className := ""
	if cell.Route == "istest" {
		for name, source := range sources {
			classes[name], className = source, name
		}
	}
	index := nativeBoundaryIndex(t, cell.API, classes, fixture.Objects)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native setup accepted; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = cell.API
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	machine := runner.newMachine()
	payload := ""
	if cell.Route == "anonymous" {
		source := sources["anonymous"]
		if source == "" {
			t.Fatal("missing exact anonymous source")
		}
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cell.API})
		if err != nil {
			t.Fatal(err)
		}
		_, execErr := machine.Execute(program)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
			t.Fatalf("missing captured assertion transport: %v", execErr)
		}
		var found bool
		_, payload, found = strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+cell.ID+"|")
		if !found {
			t.Fatalf("missing native row marker: %v", execErr)
		}
	} else {
		// Named captures measure inside @TestSetup and persist their complete
		// payload before the hosted runner invokes assertion-chunk methods.
		methods, methodErrors, programs, programErrors := compileTestSetupMethods(index)
		if methodErrors[className] != nil || programErrors[className] != nil || len(methods[className]) != 1 || len(programs[className]) != 1 {
			t.Fatalf("named capture setup compilation: %v %v", methodErrors[className], programErrors[className])
		}
		if err := machine.RegisterMethod(methods[className][0]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		if _, err := machine.ExecuteInClass(programs[className][0], className); err != nil {
			t.Fatalf("native capture setup completed; Glade failed: %v", err)
		}
		rows := 0
		for _, record := range machine.Org.Objects["Account"].Records {
			name, _ := record.GetField("Name")
			if name.String == "E51_PAYLOAD" {
				rows++
				value, _ := record.GetField("Description")
				payload = value.String
			}
		}
		if rows != 1 {
			t.Fatalf("named measurement transport rows=%d, want 1", rows)
		}
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		t.Fatalf("invalid measured payload %q: %v", payload, err)
	}
	return value
}

func eventListExtendedProjection(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	ids, uuids := map[string]int{}, map[string]int{}
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	results, exists := value["results"]
	if !exists {
		return value
	}
	rows, ok := results.([]any)
	if !ok {
		t.Fatal("measured results are not a list")
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		result, ok := row.(map[string]any)
		if !ok {
			t.Fatal("measured result is not an object")
		}
		if id, ok := result["id"].(string); ok && id != "" {
			if ids[id] == 0 {
				ids[id] = len(ids) + 1
			}
			result["id"] = fmt.Sprintf("event-identity-%d", ids[id])
		}
		failures, ok := result["errors"].([]any)
		if !ok {
			t.Fatal("measured result errors are not a list")
		}
		for _, failure := range failures {
			err, ok := failure.(map[string]any)
			if !ok {
				t.Fatal("measured error is not an object")
			}
			if err["status"] != "OPERATION_ENQUEUED" {
				continue
			}
			message, ok := err["message"].(string)
			if !ok || !uuidPattern.MatchString(message) {
				t.Fatalf("enqueue message is not a UUID: %v", err["message"])
			}
			if uuids[message] == 0 {
				uuids[message] = len(uuids) + 1
			}
			err["message"] = fmt.Sprintf("enqueue-identity-%d", uuids[message])
		}
	}
	return value
}
