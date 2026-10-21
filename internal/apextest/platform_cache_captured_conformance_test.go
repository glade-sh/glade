package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type platformCacheCapturedCell struct {
	ID, API, Route, Launcher                    string
	Variant, MeasurementRoute, MeasurementPhase string
	AllocationMB                                map[string]map[string]int64
	RequiredCapacity                            []struct {
		Partition, Scope string
		MinMB            int64 `json:"min_mb"`
	}
	PhaseSHA256 map[string]string
	Source      struct {
		Anonymous string
		Classes   map[string]string
		Tests     []string
	}
	SourceSHA256                                                                       map[string]string
	Outcome, Primary, RejectUnit, ExceptionType, Message, CarryReason, BeforeAnonymous string
	BeforeTestMethod                                                                   string
	Value                                                                              json.RawMessage
	Native                                                                             json.RawMessage
	MarkerPrefix, Flow, AsyncPrefix, PayloadField                                      string
	RollbackWriter, WriterExceptionType, WriterExceptionMessage                        string
	DelaySeconds                                                                       int64
	PairedMethods                                                                      map[string]string
}

func runPlatformCacheCapturedConformance(t *testing.T, cluster string, cells, asserted, carried int) {
	t.Helper()
	var data struct {
		APIVersions                                           []string
		Cluster                                               string
		CellCount, ObservedCount, AssertedCount, CarriedCount int
		Cells                                                 []platformCacheCapturedCell
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/platform_cache_captured", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.CellCount != cells || len(data.Cells) != cells || data.AssertedCount != asserted || data.CarriedCount != carried || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete captured matrix")
	}
	seen := map[string]bool{}
	carryCount, observed := 0, 0
	for _, cell := range data.Cells {
		key := cell.ID + "/" + cell.API + "/" + cell.Route + "/" + cell.Variant
		if seen[key] || cell.ID == "" || !json.Valid(cell.Native) {
			t.Fatalf("missing/repeated native cell: %s", key)
		}
		seen[key] = true
		if cell.Route != "anon" && cell.Route != "istest" {
			t.Fatal("invalid route")
		}
		var native struct {
			Outcome        string
			CapacityLayout struct {
				Verified                bool
				AllocationMB            map[string]map[string]int64 `json:"allocation_mb"`
				MissingRequiredCapacity []json.RawMessage           `json:"missing_required_capacity"`
			} `json:"capacity_layout"`
		}
		if err := json.Unmarshal(cell.Native, &native); err != nil {
			t.Fatal(err)
		}
		if native.Outcome != "NO_ANSWER" && native.Outcome != "FIXTURE_ERROR" {
			observed++
		}
		if cell.CarryReason != "" {
			carryCount++
		}
		if (native.Outcome == "NO_ANSWER" || native.Outcome == "FIXTURE_ERROR") && cell.CarryReason == "" {
			t.Fatal("nonanswer admitted")
		}
		values := map[string]string{}
		if cell.Source.Anonymous != "" {
			values["anonymous"] = cell.Source.Anonymous
		}
		for name, source := range cell.Source.Classes {
			values["class:"+name] = source
		}
		if len(values) == 0 || len(values) != len(cell.SourceSHA256) {
			t.Fatal("incomplete source hashes")
		}
		for name, source := range values {
			sum := sha256.Sum256([]byte(source))
			if source == "" || hex.EncodeToString(sum[:]) != cell.SourceSHA256[name] {
				t.Fatalf("captured source changed: %s", key)
			}
		}
		if cell.Variant == "retry" || cell.Variant == "extended" {
			if !native.CapacityLayout.Verified || !reflect.DeepEqual(cell.AllocationMB, native.CapacityLayout.AllocationMB) || len(native.CapacityLayout.MissingRequiredCapacity) != 0 {
				t.Fatal("retry layout disagrees with native capacity evidence")
			}
			if len(cell.AllocationMB) != 3 {
				t.Fatal("retry requires its captured capacity layout")
			}
			total := int64(0)
			for _, name := range []string{"PcMain", "PcAlt", "PcZero"} {
				if len(cell.AllocationMB[name]) != 2 {
					t.Fatal("incomplete captured cache scopes")
				}
				for _, scope := range []string{"Organization", "Session"} {
					capacity, exists := cell.AllocationMB[name][scope]
					if !exists || capacity < 0 || capacity > 1 {
						t.Fatal("uncaptured allocation")
					}
					total += capacity
				}
			}
			if total > 3 {
				t.Fatal("retry exceeds captured three-MB org limit")
			}
			for _, required := range cell.RequiredCapacity {
				if required.MinMB <= 0 || cell.AllocationMB[required.Partition][required.Scope] < required.MinMB {
					t.Fatal("captured row lacks required capacity")
				}
			}
			if cell.BeforeAnonymous != "" && cell.Variant == "retry" {
				sum := sha256.Sum256([]byte(cell.BeforeAnonymous))
				if cell.PhaseSHA256["beforeAnonymous"] != hex.EncodeToString(sum[:]) {
					t.Fatal("captured anonymous writer changed")
				}
			}
		}
		if cell.Variant == "extended" {
			for name, source := range map[string]string{"BeforeAnonymous": cell.BeforeAnonymous, "RollbackWriter": cell.RollbackWriter} {
				if source != "" {
					sum := sha256.Sum256([]byte(source))
					if cell.PhaseSHA256[name] != hex.EncodeToString(sum[:]) {
						t.Fatal("captured request phase changed")
					}
				}
			}
		}
		if cell.MeasurementRoute != "" && cell.MeasurementRoute != cell.Route &&
			!(cell.Variant == "retry" && cell.Route == "istest" && cell.MeasurementRoute == "anon" && cell.MeasurementPhase == "anonymous-reader") {
			t.Fatal("unexpected measurement route")
		}
		if cell.CarryReason == "" {
			switch cell.Outcome {
			case "value":
				if !json.Valid(cell.Value) {
					t.Fatal("missing captured value")
				}
			case "reject":
				if cell.Primary == "" || cell.RejectUnit == "" {
					t.Fatal("empty native rejection")
				}
			case "uncaught":
				if cell.ExceptionType == "" || cell.Message == "" {
					t.Fatal("empty native exception")
				}
			default:
				t.Fatal("nonanswer admitted")
			}
		}
	}
	if carryCount != carried || observed != data.ObservedCount {
		t.Fatal("incorrect observation counts")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "PlatformCacheCaptured-"+cluster, api)
			baseIndex := platformCacheCapturedIndex(t, api, nil)
			org := platformCacheCapturedOrg(t, baseIndex, api, cluster)
			// Link the same passive platform surface as Run. Without it a
			// shadowed call could fall through to the builtin after lookup fails,
			// hiding a captured dispatch failure.
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for position, cell := range data.Cells {
				if cell.API != api {
					continue
				}
				name := fmt.Sprintf("case-%03d/%s", position+1, cell.Route)
				if cell.Variant != "" {
					name += "/" + cell.Variant
				}
				t.Run(name, func(t *testing.T) {
					if cell.CarryReason != "" {
						counts.TrackKind(t, cell.Route, "carried", 1)
						t.Logf("carried: %s; native=%s", cell.CarryReason, cell.Native)
						return
					}
					kind := "exact"
					if cell.Outcome == "reject" {
						kind = "category"
					}
					counts.TrackKind(t, cell.Route, kind, 1)
					index := platformCacheCapturedIndex(t, api, cell.Source.Classes)
					rowOrg := org
					if cell.AllocationMB != nil {
						rowOrg = platformCacheCapturedOrg(t, baseIndex, api, cluster, cell.AllocationMB)
					}
					analysis := sema.Analyze(index)
					diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
					var compileErr, execErr error
					measurementRoute := cell.Route
					if cell.MeasurementRoute != "" {
						measurementRoute = cell.MeasurementRoute
					}
					if measurementRoute == "anon" {
						anonymousAnalysis := sema.AnalyzeAnonymous(index, cell.Source.Anonymous, api)
						diagnostics = append(diagnostics, anonymousAnalysis.Diagnostics...)
						program, err := vm.CompileAnonymousWithOptions(cell.Source.Anonymous, vm.CompileOptions{APIVersion: api})
						compileErr = err
						platformCacheCapturedCompile(t, cell, diagnostics, compileErr)
						if cell.Outcome == "reject" {
							return
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &rowOrg})
						if platformCacheCapturedLifecycle(t, runner, index, cell, api, program) {
							return
						}
						platformCacheCapturedBefore(t, runner, index, cell, api)
						_, execErr = runner.execute(program)
					} else {
						platformCacheCapturedCompile(t, cell, diagnostics, nil)
						if cell.Outcome == "reject" {
							return
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &rowOrg})
						if platformCacheCapturedLifecycle(t, runner, index, cell, api, ir.Program{}) {
							return
						}
						platformCacheCapturedBefore(t, runner, index, cell, api)
						execErr = platformCacheCapturedNamed(t, runner, index, cell.Source.Tests, "observe")
					}
					platformCacheCapturedValue(t, cell, execErr)
				})
			}
		})
	}
}

// The capture launched each writer and reader as separate requests. Execute
// neither on the linked base nor on the following reader's private VM.
func platformCacheCapturedBefore(t *testing.T, runner *conformanceRunner, index typesys.Index, cell platformCacheCapturedCell, api string) {
	t.Helper()
	if cell.BeforeAnonymous != "" {
		if analysis := sema.AnalyzeAnonymous(index, cell.BeforeAnonymous, api); analysis.HasErrors() {
			t.Fatalf("anonymous writer semantics: %v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(cell.BeforeAnonymous, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runner.execute(program); err != nil {
			t.Fatalf("anonymous writer failed: %v", err)
		}
	}
	if cell.BeforeTestMethod != "" {
		if err := platformCacheCapturedNamed(t, runner, index, cell.Source.Tests, cell.BeforeTestMethod); err != nil {
			t.Fatalf("test writer failed: %v", err)
		}
	}
}

func platformCacheCapturedNamed(t *testing.T, runner *conformanceRunner, index typesys.Index, tests []string, method string) error {
	t.Helper()
	cases := Discover(index, Options{SelectedClasses: tests, SelectedMethod: method})
	if len(cases) != 1 {
		t.Fatalf("named %s discovery=%d", method, len(cases))
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	machine := runner.newMachine()
	setups, setupErrors, setupPrograms, setupProgramErrors := compileTestSetupMethodsForClasses(index, testCaseClassSet(cases), newSourceCache())
	className := cases[0].ClassName
	if len(setups[className]) != 0 {
		if err := registerTestRuntime(runner.base, flattenSetupMethods(setups)); err != nil {
			t.Fatal(err)
		}
		runner.base.FreezeClassLookup()
		seed, random, err, _ := prepareTestSetupOrg(context.Background(), className, runner.base, nil, setups[className], setupErrors[className], setupPrograms[className], setupProgramErrors[className], nil, runner.newOrg(), Options{Parallelism: 1}, nil)
		if err != nil {
			t.Fatalf("test setup failed: %v", err)
		}
		machine = runner.newMachine()
		machine.SetRuntimeTemplateOrg(&seed)
		machine.SetDeterministicRandomState(random)
	}
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	machine.SetTestSeeAllData(cases[0].SeeAllData)
	_, err := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
	return err
}

func platformCacheCapturedIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
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

func platformCacheCapturedOrg(t *testing.T, index typesys.Index, api, cluster string, layouts ...map[string]map[string]int64) storage.OrgState {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = api
	// Match retrieved setup, never the planned four-MB layout or local defaults.
	for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
		storage.EnsureStandardObject(&org, name)
		object := org.Objects[name]
		object.Records = map[storage.ID]storage.Record{}
		org.Objects[name] = object
	}
	if cluster == "no_partition" || cluster == "bare_controls" {
		return org
	}
	partitions := org.Objects["PlatformCachePartition"]
	capacities := org.Objects["PlatformCachePartitionType"]
	for i, name := range []string{"PcMain", "PcAlt", "PcZero"} {
		id := storage.ID(fmt.Sprintf("0Px000000000%03d", i+101))
		partitions.Records[id] = storage.Record{ID: id, Object: "PlatformCachePartition", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue(name), "NamespacePrefix": storage.StringValue(""), "IsDefaultPartition": storage.BooleanValue(name == "PcMain")}}
		for j, scope := range []string{"Organization", "Session"} {
			capacity := int64(0)
			if name == "PcAlt" {
				capacity = 1
			}
			if len(layouts) != 0 {
				capacity = layouts[0][name][scope]
			}
			capacityID := storage.ID(fmt.Sprintf("0Pt000000000%03d", i*2+j+101))
			capacities.Records[capacityID] = storage.Record{ID: capacityID, Object: "PlatformCachePartitionType", Fields: map[string]storage.Value{"PlatformCachePartitionId": storage.IDValue(id), "CacheType": storage.StringValue(scope), "AllocatedCapacity": storage.IntegerValue(capacity)}}
		}
	}
	org.Objects["PlatformCachePartition"] = partitions
	org.Objects["PlatformCachePartitionType"] = capacities
	return org
}

func platformCacheCapturedCompile(t *testing.T, cell platformCacheCapturedCell, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var rejected bool
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if cell.Outcome != "reject" {
			t.Fatalf("native accepts; Glade rejects: %v", d)
		}
		if cell.RejectUnit == "anonymous" || strings.EqualFold(filepath.Base(d.File), cell.RejectUnit+".cls") {
			rejected = true
		}
	}
	if cell.Outcome == "reject" {
		if cell.RejectUnit == "anonymous" && compileErr != nil {
			rejected = true
		}
		if !rejected {
			t.Fatalf("native rejects %s: <%s>; Glade diagnostics=%v compile=%v", cell.RejectUnit, cell.Primary, diagnostics, compileErr)
		}
		return
	}
	if compileErr != nil {
		t.Fatal(compileErr)
	}
}

func platformCacheCapturedQualifiedType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func platformCacheCapturedValue(t *testing.T, cell platformCacheCapturedCell, err error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("missing native observation: %v", err)
	}
	message := runtimeErr.ExceptionMessage()
	if cell.Outcome == "uncaught" {
		if platformCacheCapturedQualifiedType(runtimeErr.Type) != cell.ExceptionType || message != cell.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", cell.ExceptionType, cell.Message, runtimeErr.Type, message)
		}
		return
	}
	if platformCacheCapturedQualifiedType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing assertion envelope: %v", err)
	}
	prefix := cell.MarkerPrefix
	if prefix == "" {
		// Older fixtures embed their transport prefix in the captured source.
		sources := []string{cell.Source.Anonymous}
		for _, source := range cell.Source.Classes {
			sources = append(sources, source)
		}
		for _, source := range sources {
			if match := regexp.MustCompile(`'([A-Z0-9_]+_RESULT\|)`).FindStringSubmatch(source); len(match) == 2 {
				prefix = match[1]
				break
			}
		}
	}
	marker := prefix + cell.ID + "|"
	_, payload, ok := strings.Cut(message, marker)
	if prefix == "" || !ok || strings.Count(message, prefix) != 1 {
		t.Fatalf("invalid marker: %s", message)
	}
	platformCacheCapturedPayload(t, cell, payload)
}

func platformCacheCapturedPayload(t *testing.T, cell platformCacheCapturedCell, payload string) {
	t.Helper()
	if cell.PayloadField != "" {
		var envelope string
		if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(envelope), &fields); err != nil {
			t.Fatal(err)
		}
		projected, exists := fields[cell.PayloadField]
		if !exists {
			t.Fatal("missing captured payload field")
		}
		payload = string(projected)
	}
	var got, want *string
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cell.Value, &want); err != nil {
		t.Fatal(err)
	}
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Fatalf("native <%s>; Glade <%s>", cell.Value, payload)
		}
		return
	}
	qualify := func(value string) string {
		parts := strings.SplitN(value, "|", 3)
		if len(parts) == 3 && parts[0] == "EXC" {
			parts[1] = platformCacheCapturedQualifiedType(parts[1])
			return strings.Join(parts, "|")
		}
		return value
	}
	if qualify(*got) != qualify(*want) {
		t.Fatalf("native <%s>; Glade <%s>", *want, *got)
	}
}

func TestPlatformCacheCapturedNoPartitionConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "no_partition", 84, 84, 0)
}
func TestPlatformCacheCapturedAvailabilityConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "availability", 117, 111, 6)
}
func TestPlatformCacheCapturedShadowBoundariesConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "shadow_boundaries", 42, 36, 6)
}
func TestPlatformCacheCapturedTestContextConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "test_context", 33, 33, 0)
}
func TestPlatformCacheCapturedBareControlsConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "bare_controls", 18, 18, 0)
}
func TestPlatformCacheCapturedConfiguredControlsConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "configured_controls", 18, 18, 0)
}

func TestPlatformCacheCapturedRetryControlsConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "retry_controls", 18, 18, 0)
}

func TestPlatformCacheCapturedExtendedControlsConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "extended_controls", 18, 18, 0)
}
func TestPlatformCacheCapturedRequestNamesConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "request_names", 48, 30, 18)
}
func TestPlatformCacheCapturedLexicalNamesConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "lexical_names", 84, 72, 12)
}
func TestPlatformCacheCapturedKeyBoundariesConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "key_boundaries", 138, 138, 0)
}
func TestPlatformCacheCapturedAsyncContextConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "async_context", 48, 48, 0)
}
func TestPlatformCacheCapturedTransactionBoundariesConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "transaction_boundaries", 30, 27, 3)
}
func TestPlatformCacheCapturedRequestPersistenceConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "request_persistence", 18, 18, 0)
}
func TestPlatformCacheCapturedVisibilityConformance(t *testing.T) {
	runPlatformCacheCapturedConformance(t, "visibility", 12, 12, 0)
}
