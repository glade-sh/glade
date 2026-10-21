package apextest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const metadataNamespaceFixture = "testdata/conformance/custom_metadata_namespace"

type metadataNamespaceCell struct {
	API, Route, Source, SourceSHA256, ClassName, Outcome, Expected, PrimaryDiagnostic string
	Deployment                                                                        *struct {
		Deployment struct {
			Status                string
			Success, CheckOnly    bool
			NumberComponentErrors int
			ComponentFailures     []struct{ Problem string }
		}
		Records []map[string]any
	}
}
type metadataNamespaceRow struct {
	ID, Group, Kind, Twin string
	Cells                 []metadataNamespaceCell
}
type metadataNamespaceData struct {
	APIVersions, Routes                []string
	ShapeCount, NativeObservationCount int
	Files                              []struct{ Path, SHA256 string }
	Rows                               []metadataNamespaceRow
}

func TestCustomMetadataRecordOwnershipOrgConformance(t *testing.T) {
	runMetadataNamespaceConformance(t, "ownership", 16)
}
func TestCustomMetadataGettersOrgConformance(t *testing.T) {
	runMetadataNamespaceConformance(t, "getters", 7)
}
func TestCustomMetadataNamespaceDescribeOrgConformance(t *testing.T) {
	runMetadataNamespaceConformance(t, "describe", 3)
}
func TestCustomMetadataIdentityDTOOrgConformance(t *testing.T) {
	runMetadataNamespaceConformance(t, "identity_dto", 4)
}
func TestCustomMetadataDeploymentOwnershipOrgConformance(t *testing.T) {
	runMetadataNamespaceConformance(t, "deployment", 2)
}

// The shared conformanceRunner supplies a frozen API base and fresh VM/org per
// cell. Captured Apex bytes, route and source API remain separate dimensions.
// This is a source dependency representation of the installed CMT, not a 1GP
// package-manager implementation or an oracle for package lifecycle commands.
func runMetadataNamespaceConformance(t *testing.T, group string, shapes int) {
	t.Helper()
	var data metadataNamespaceData
	raw, err := os.ReadFile(filepath.Join(metadataNamespaceFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 32 || data.NativeObservationCount != 192 || len(data.Rows) != 32 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatal("incomplete native matrix")
	}
	listed := map[string]bool{}
	for _, file := range data.Files {
		if listed[file.Path] || filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
			t.Fatal("invalid native fixture file")
		}
		listed[file.Path] = true
		metadataNamespaceRead(t, file.Path, file.SHA256)
	}
	seen := map[string]bool{}
	rows := []metadataNamespaceRow{}
	for _, row := range data.Rows {
		if seen[row.ID] || row.ID == "" || len(row.Cells) != 6 {
			t.Fatal("invalid native row")
		}
		seen[row.ID] = true
		matrix := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if matrix[key] || !listed[cell.Source] || (cell.Route != "anonymous" && cell.Route != "isTest") || (cell.API != "62.0" && cell.API != "67.0" && cell.API != "68.0") {
				t.Fatal("invalid native cell")
			}
			matrix[key] = true
			metadataNamespaceRead(t, cell.Source, cell.SourceSHA256)
		}
		if row.Group == group {
			rows = append(rows, row)
		}
	}
	for _, row := range data.Rows {
		if !seen[row.Twin] {
			t.Fatal("missing captured twin")
		}
	}
	if len(rows) != shapes {
		t.Fatal("incomplete native cluster")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "CustomMetadataNamespace_"+group, api)
			baseIndex := metadataNamespaceIndex(t, api, nil, data)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			if org.Namespace != "" || len(org.Objects["gladens001__Ns113bFlag__mdt"].Records) != 4 || len(org.Objects["Ns113bLocal__mdt"].Records) != 1 {
				t.Fatal("synthetic subscriber fixture failed to load all five native setup records")
			}
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							counts.TrackKind(t, route, "exact", 1)
							cell := metadataNamespaceFind(t, row, api, route)
							if route == "anonymous" {
								metadataNamespaceAnonymous(t, data, row, cell, base)
							} else {
								metadataNamespaceNamed(t, data, row, cell, base)
							}
						})
					}
				})
			}
		})
	}
}

func metadataNamespaceRead(t *testing.T, name, digest string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(metadataNamespaceFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		t.Fatalf("native source/evidence changed: %s", name)
	}
	return string(raw)
}
func metadataNamespaceFind(t *testing.T, row metadataNamespaceRow, api, route string) metadataNamespaceCell {
	t.Helper()
	for _, cell := range row.Cells {
		if cell.API == api && cell.Route == route {
			return cell
		}
	}
	t.Fatal("missing native cell")
	return metadataNamespaceCell{}
}
func metadataNamespaceIndex(t *testing.T, api string, classes map[string]string, data metadataNamespaceData) typesys.Index {
	t.Helper()
	root := t.TempDir()
	for _, file := range data.Files {
		if !strings.HasPrefix(file.Path, "fixture/") {
			continue
		}
		text := metadataNamespaceRead(t, file.Path, file.SHA256)
		// Native caller source API varies; package metadata stays uploaded at68.
		if file.Path == "fixture/subscriber/sfdx-project.json" {
			var config map[string]any
			if err := json.Unmarshal([]byte(text), &config); err != nil {
				t.Fatal(err)
			}
			config["sourceApiVersion"] = api
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			text = string(raw)
		}
		writeFile(t, filepath.Join(root, strings.TrimPrefix(file.Path, "fixture/")), text)
	}
	for name, text := range classes {
		path := filepath.Join(root, "subscriber/force-app/main/default/classes", name+".cls")
		writeFile(t, path, text)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, filepath.Join(root, "subscriber"))
}
func metadataNamespaceErrors(items []diagnostic.Diagnostic) bool {
	for _, item := range items {
		if item.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}
func metadataNamespaceCompile(t *testing.T, cell metadataNamespaceCell, items []diagnostic.Diagnostic, compileErr error) bool {
	t.Helper()
	if cell.Outcome == "compile-rejected" {
		primary := ""
		for _, item := range items {
			if item.Severity == diagnostic.Error {
				primary = item.NativeMessage
				if primary == "" {
					primary = item.Message
				}
				break
			}
		}
		if primary == "" && compileErr != nil {
			primary = compileErr.Error()
		}
		if primary != cell.PrimaryDiagnostic {
			t.Fatalf("native primary <%s>; Glade <%s> diagnostics=%v compile=%v", cell.PrimaryDiagnostic, primary, items, compileErr)
		}
		return false
	}
	if metadataNamespaceErrors(items) || compileErr != nil {
		t.Fatalf("native compilation accepted; diagnostics=%v compile=%v", items, compileErr)
	}
	return cell.Outcome != "compiled"
}
func metadataNamespaceAnonymous(t *testing.T, data metadataNamespaceData, row metadataNamespaceRow, cell metadataNamespaceCell, base *conformanceRunner) {
	t.Helper()
	source := metadataNamespaceRead(t, cell.Source, cell.SourceSHA256)
	declarations, body := apexMetadataDeclarations(t, source)
	index := sema.WithAnonymousDeclarationContext(metadataNamespaceIndex(t, cell.API, declarations, data))
	diagnostics := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
	if !metadataNamespaceErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
	}
	if !metadataNamespaceErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, body, cell.API).Diagnostics...)
	}
	program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: cell.API, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, cell.API)})
	if !metadataNamespaceCompile(t, cell, diagnostics, compileErr) {
		return
	}
	runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	org := base.newOrg()
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &org})
	machine := runner.newMachine()
	if cell.Outcome == "deployment" {
		// E002's native readback is E001's preserved record. Keep that sequence on
		// this private cell VM; do not fabricate a separately observed baseline.
		if row.ID == "E002" {
			for _, seed := range data.Rows {
				if seed.ID == "E001" {
					prior := metadataNamespaceFind(t, seed, cell.API, "anonymous")
					_, warmup := apexMetadataDeclarations(t, metadataNamespaceRead(t, prior.Source, prior.SourceSHA256))
					prep, e := vm.CompileAnonymousWithOptions(warmup, vm.CompileOptions{APIVersion: cell.API})
					if e != nil {
						t.Fatal(e)
					}
					result, e := machine.Execute(prep)
					if e != nil {
						t.Fatal(e)
					}
					metadataNamespaceEnqueueID(t, seed.ID, result)
				}
			}
		}
		result, e := machine.Execute(program)
		if e != nil {
			t.Fatal(e)
		}
		metadataNamespaceDeployment(t, row.ID, cell, machine, result)
		return
	}
	_, execErr := machine.Execute(program)
	metadataNamespaceObservation(t, row.ID, cell, execErr)
}
func metadataNamespaceNamed(t *testing.T, data metadataNamespaceData, row metadataNamespaceRow, cell metadataNamespaceCell, base *conformanceRunner) {
	t.Helper()
	source := metadataNamespaceRead(t, cell.Source, cell.SourceSHA256)
	index := metadataNamespaceIndex(t, cell.API, map[string]string{cell.ClassName: source}, data)
	diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
	if cell.Outcome == "compile-rejected" {
		metadataNamespaceCompile(t, cell, diagnostics, nil)
		return
	}
	if metadataNamespaceErrors(diagnostics) {
		t.Fatalf("native named compilation accepted; %v", diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: []string{cell.ClassName}, SelectedMethod: "observed"})
	if len(cases) != 1 {
		t.Fatal("one captured test method required")
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	if cell.Outcome == "compiled" {
		return
	}
	org := base.newOrg()
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
	machine := runner.newMachine()
	machine.EnableTestContext()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	metadataNamespaceObservation(t, row.ID, cell, execErr)
}
func metadataNamespaceObservation(t *testing.T, id string, cell metadataNamespaceCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("captured observation transport missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	observed := ""
	if cell.Route == "anonymous" {
		if runtimeErr.Type != "FamilyProbeTransportException" && !strings.HasSuffix(runtimeErr.Type, ".FamilyProbeTransportException") {
			t.Fatalf("wrong observation transport: %v", execErr)
		}
		_, text, ok := strings.Cut(message, "GLADE_FAMILY_ROWS|")
		if !ok {
			t.Fatal("missing captured anonymous marker")
		}
		var lines []string
		if err := json.Unmarshal([]byte(text), &lines); err != nil || len(lines) != 1 {
			t.Fatalf("invalid row transport: %s", text)
		}
		var found bool
		_, observed, found = strings.Cut(lines[0], "P|"+id+"|")
		if !found {
			t.Fatal("wrong anonymous row marker")
		}
	} else {
		if runtimeErr.Type != "System.AssertException" {
			t.Fatalf("wrong named observation transport: %v", execErr)
		}
		_, text, ok := strings.Cut(message, "P|"+id+"|")
		if !ok {
			t.Fatal("missing captured named marker")
		}
		observed = text
	}
	if observed != cell.Expected {
		t.Fatalf("native <%s>; Glade <%s>", cell.Expected, observed)
	}
}
func metadataNamespaceEnqueueID(t *testing.T, id string, result vm.Result) string {
	t.Helper()
	prefix := "P64|" + id + "|"
	request := ""
	for _, event := range result.DebugEvents {
		if !strings.HasPrefix(event.Message, prefix) {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(event.Message, prefix))
		if err != nil {
			t.Fatal(err)
		}
		if request != "" {
			t.Fatal("duplicate enqueue observation")
		}
		request = string(raw)
	}
	if request == "" || strings.HasPrefix(request, "EXC|") {
		t.Fatalf("native enqueue accepted; local observation <%s>", request)
	}
	return request
}
func metadataNamespaceDeployment(t *testing.T, id string, cell metadataNamespaceCell, machine *vm.VM, result vm.Result) {
	t.Helper()
	request := metadataNamespaceEnqueueID(t, id, result)
	if cell.Deployment == nil {
		t.Fatal("missing captured deployment/readback")
	}
	// LOCAL readback adapter: native status came from CLI deploy-report, not an
	// extra Apex checkDeployStatus oracle. No callback is added to captured code.
	observer := fmt.Sprintf("Metadata.DeployResult replayResult=Metadata.Operations.checkDeployStatus(%s);", conformanceApexString(request))
	program, err := vm.CompileAnonymousWithOptions(observer, vm.CompileOptions{APIVersion: cell.API})
	if err != nil {
		t.Fatal(err)
	}
	status, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	value := status.Vars["replayResult"]
	want := cell.Deployment.Deployment
	if value.Fields["status"].Text != want.Status || value.Fields["success"].Bool != want.Success || value.Fields["checkOnly"].Bool != want.CheckOnly || value.Fields["numberComponentErrors"].Int != int64(want.NumberComponentErrors) {
		t.Fatalf("native deploy status=%s success=%v checkOnly=%v errors=%d; local=%v", want.Status, want.Success, want.CheckOnly, want.NumberComponentErrors, value)
	}
	if len(want.ComponentFailures) > 0 {
		failure := ""
		for _, event := range result.Trace {
			if event.Name == "apex.metadata.deploy.enqueue" {
				failure, _ = event.Args["error"].(string)
			}
		}
		if failure != want.ComponentFailures[0].Problem {
			t.Fatalf("native component failure <%s>; local <%s>", want.ComponentFailures[0].Problem, failure)
		}
	}
	name := "Apex" + strings.TrimSuffix(cell.API, ".0")
	actual := []map[string]any{}
	for _, record := range machine.Org.Objects["gladens001__Ns113bFlag__mdt"].Records {
		developer, _ := record.GetField("DeveloperName")
		if developer.String != name {
			continue
		}
		observed := map[string]any{}
		for _, field := range []string{"DeveloperName", "NamespacePrefix", "QualifiedApiName", "gladens001__Enabled__c"} {
			v, ok := record.GetField(field)
			if !ok {
				t.Fatalf("missing native readback field %s", field)
			}
			switch v.Kind {
			case storage.ValueNull:
				observed[field] = nil
			case storage.ValueString:
				observed[field] = v.String
			case storage.ValueBoolean:
				observed[field] = v.Boolean
			default:
				t.Fatalf("unexpected readback field kind %s", v.Kind)
			}
		}
		actual = append(actual, observed)
	}
	sort.Slice(actual, func(i, j int) bool {
		return fmt.Sprint(actual[i]["DeveloperName"]) < fmt.Sprint(actual[j]["DeveloperName"])
	})
	if !reflect.DeepEqual(actual, cell.Deployment.Records) {
		t.Fatalf("native deployed/preserved record=%v; local=%v", cell.Deployment.Records, actual)
	}
	if id == "E002" {
		if _, exists := machine.Org.Objects["ns113wrong__Ns113bAbsent__mdt"]; exists {
			t.Fatal("native missing-type rejection must not synthesize a type")
		}
	}
}
