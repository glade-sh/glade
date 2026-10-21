package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type picklistOrderNativeFixture struct {
	ProjectAPI, ProjectSource, ProjectSourceSHA256                   string
	OrgIDSHA256, API68OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256        string
	RowCount, ScalarCells, MatchedRows, CarriedRows, ExpectedMethods int
	Sources, SourceSHA256                                            map[string]string
	MetadataFiles, MetadataSHA256                                    map[string]string
	AnonymousSource, AnonymousSourceSHA256                           string
	OrgContext                                                       struct {
		RequestedEdition, OtherFeatureContext string
		FeaturesRequested                     []string
		SettingsChanged                       *bool
	}
	Captures []struct {
		API, TransportAPI, OrgIDSHA256, Metadata, MetadataSHA256, DeploymentID, DeploymentStatus, DeploymentReceiptSHA256 string
		CheckOnly                                                                                                         bool
	}
	Rows []struct {
		API, TransportAPI, Route, ID, ReceiptSHA256 string
		Native                                      map[string]any
	}
	Diagnostics []struct {
		API, TransportAPI, Route, Source, SourceSHA256, Metadata, MetadataSHA256, ReceiptSHA256, CarryReason string
		Rows                                                                                                 []struct {
			ID     string
			Native map[string]any
		}
	}
	VersionSources, VersionSourceSHA256                                      map[string]string
	VersionAnonymousSource, VersionAnonymousSourceSHA256, VersionOrgIDSHA256 string
	VersionProbeRowCount, VersionProbeScalarCells                            int
	VersionProbes                                                            []struct {
		API, TransportAPI, Route, Metadata, MetadataSHA256, DeploymentID, DeploymentStatus, DeploymentReceiptSHA256, ReceiptSHA256 string
		CheckOnly                                                                                                                  bool
		Rows                                                                                                                       []struct {
			ID     string
			Native map[string]any
		}
	}
	Uncaptured []string
}

func loadPicklistOrderNativeFixture(t *testing.T) picklistOrderNativeFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/picklist_order_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var data picklistOrderNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func picklistOrderNativeIDs() []string {
	ids := []string{"M01"}
	for _, prefix := range []string{"K", "U"} {
		for n := 1; n <= 12; n++ {
			ids = append(ids, fmt.Sprintf("%s%02d", prefix, n))
		}
	}
	return ids
}

func TestPicklistOrderNativeFixture(t *testing.T) {
	data := loadPicklistOrderNativeFixture(t)
	digest := func(value string) {
		t.Helper()
		if decoded, err := hex.DecodeString(value); err != nil || len(decoded) != sha256.Size {
			t.Fatal("invalid native receipt binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		hash := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(hash[:]) != want {
			t.Fatal("native source binding changed")
		}
	}
	if data.RowCount != 150 || len(data.Rows) != 150 || data.ScalarCells != 714 || data.MatchedRows != 150 || data.CarriedRows != 0 || data.ExpectedMethods != 25 {
		t.Fatal("native picklist matrix contract changed")
	}
	if len(data.Sources) != 2 || len(data.SourceSHA256) != 2 || len(data.Captures) != 3 {
		t.Fatal("incomplete native source or capture matrix")
	}
	for _, name := range []string{"OwnedPicklistOrderObserver", "OwnedPicklistOrderTest"} {
		checkHash(data.Sources[name], data.SourceSHA256[name])
	}
	metadataPaths := []string{
		"force-app/main/default/objects/OwnedSortEntry__c/OwnedSortEntry__c.object-meta.xml",
		"force-app/main/default/objects/OwnedSortEntry__c/fields/Text__c.field-meta.xml",
		"force-app/main/default/objects/OwnedSortEntry__c/fields/Stage__c.field-meta.xml",
		"force-app/main/default/objects/OwnedSortEntry__c/fields/Parent__c.field-meta.xml",
	}
	if len(data.MetadataFiles) != len(metadataPaths) || len(data.MetadataSHA256) != len(metadataPaths) {
		t.Fatal("native object metadata matrix changed")
	}
	for _, path := range metadataPaths {
		checkHash(data.MetadataFiles[path], data.MetadataSHA256[path])
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, value := range []string{data.OrgIDSHA256, data.API68OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		digest(value)
	}
	checkHash(data.AnonymousSource, data.AnonymousSourceSHA256)
	checkHash(data.ProjectSource, data.ProjectSourceSHA256)
	if data.API68OrgIDSHA256 == data.OrgIDSHA256 || data.API68OrgIDSHA256 == data.VersionOrgIDSHA256 {
		t.Fatal("native API 68 org context changed")
	}
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(data.ProjectSource), &project) != nil || data.ProjectAPI != "62.0" || project.API != data.ProjectAPI {
		t.Fatal("native project API pin changed")
	}
	seenAPI := make(map[string]bool)
	for _, capture := range data.Captures {
		if (capture.API != "62.0" && capture.API != "67.0" && capture.API != "68.0") || seenAPI[capture.API] || capture.API != capture.TransportAPI || capture.CheckOnly || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" {
			t.Fatal("invalid native capture")
		}
		seenAPI[capture.API] = true
		digest(capture.OrgIDSHA256)
		wantOrg := data.OrgIDSHA256
		if capture.API == "68.0" {
			wantOrg = data.API68OrgIDSHA256
		}
		if capture.OrgIDSHA256 != wantOrg {
			t.Fatal("native capture org context changed")
		}
		checkHash(capture.Metadata, capture.MetadataSHA256)
		digest(capture.DeploymentReceiptSHA256)
		if !strings.Contains(capture.Metadata, "<apiVersion>"+capture.API+"</apiVersion>") {
			t.Fatal("native component API changed")
		}
	}
	wanted := make(map[string]bool)
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, id := range picklistOrderNativeIDs() {
				wanted[api+"/"+route+"/"+id] = true
			}
		}
	}
	cells := 0
	for _, row := range data.Rows {
		key := row.API + "/" + row.Route + "/" + row.ID
		if !wanted[key] || !seenAPI[row.API] || row.API != row.TransportAPI || row.Native == nil {
			t.Fatal("unexpected, duplicate or incomplete native row", key)
		}
		delete(wanted, key)
		digest(row.ReceiptSHA256)
		cells += roleGroupObservationLeaves(row.Native)
	}
	if len(wanted) != 0 || cells != data.ScalarCells {
		t.Fatalf("native coverage changed: missing=%d cells=%d", len(wanted), cells)
	}
	if len(data.Diagnostics) != 2 {
		t.Fatal("native default-access diagnostic contexts missing")
	}
	seenDiagnosticRoute := make(map[string]bool)
	seenDiagnosticSource := make(map[string]bool)
	for _, diagnostic := range data.Diagnostics {
		if diagnostic.API != "67.0" || diagnostic.TransportAPI != "67.0" || (diagnostic.Route != "istest" && diagnostic.Route != "anonymous") || seenDiagnosticRoute[diagnostic.Route] || strings.TrimSpace(diagnostic.CarryReason) == "" {
			t.Fatal("invalid native default-access diagnostic context")
		}
		seenDiagnosticRoute[diagnostic.Route] = true
		checkHash(diagnostic.Source, diagnostic.SourceSHA256)
		checkHash(diagnostic.Metadata, diagnostic.MetadataSHA256)
		digest(diagnostic.ReceiptSHA256)
		if diagnostic.SourceSHA256 == data.SourceSHA256["OwnedPicklistOrderObserver"] || seenDiagnosticSource[diagnostic.SourceSHA256] || !strings.Contains(diagnostic.Metadata, "<apiVersion>67.0</apiVersion>") {
			t.Fatal("native default-access source context changed")
		}
		seenDiagnosticSource[diagnostic.SourceSHA256] = true
		wantedDiagnostics := make(map[string]bool)
		exceptionType := "System.QueryException"
		if diagnostic.Route == "istest" {
			exceptionType = "System.DmlException"
			for _, id := range picklistOrderNativeIDs() {
				wantedDiagnostics[id] = true
			}
		} else {
			wantedDiagnostics["M01"] = true
		}
		if len(diagnostic.Rows) != len(wantedDiagnostics) {
			t.Fatal("native default-access diagnostic row count changed")
		}
		for _, row := range diagnostic.Rows {
			if !wantedDiagnostics[row.ID] || row.Native["exceptionType"] != exceptionType {
				t.Fatal("invalid native default-access diagnostic row", row.ID)
			}
			delete(wantedDiagnostics, row.ID)
			message, ok := row.Native["message"].(string)
			if !ok || strings.TrimSpace(message) == "" {
				t.Fatal("missing native default-access diagnostic message", row.ID)
			}
			t.Logf("DIAGNOSTIC CARRY %s/%s/%s native=%v reason=%s", diagnostic.API, diagnostic.Route, row.ID, row.Native, diagnostic.CarryReason)
		}
		if len(wantedDiagnostics) != 0 {
			t.Fatal("incomplete native default-access diagnostic matrix")
		}
	}
	if len(data.VersionSources) != 2 || len(data.VersionSourceSHA256) != 2 || data.VersionSources["OwnedPicklistOrderObserver"] != data.Sources["OwnedPicklistOrderObserver"] {
		t.Fatal("native source-policy observer context changed")
	}
	for _, name := range []string{"OwnedPicklistOrderObserver", "OwnedPicklistVersionTest"} {
		checkHash(data.VersionSources[name], data.VersionSourceSHA256[name])
	}
	checkHash(data.VersionAnonymousSource, data.VersionAnonymousSourceSHA256)
	digest(data.VersionOrgIDSHA256)
	if data.VersionOrgIDSHA256 == data.OrgIDSHA256 || data.VersionProbeRowCount != 35 || data.VersionProbeScalarCells != 189 || len(data.VersionProbes) != 7 {
		t.Fatal("native source-policy matrix context changed")
	}
	wantedVersionContexts := map[string]bool{
		"62.0/67.0/istest":    true,
		"62.0/67.0/anonymous": true,
		"67.0/62.0/istest":    true,
		"67.0/62.0/anonymous": true,
		"64.0/67.0/istest":    true,
		"65.0/67.0/istest":    true,
		"66.0/67.0/istest":    true,
	}
	versionRows, versionCells := 0, 0
	for _, probe := range data.VersionProbes {
		key := probe.API + "/" + probe.TransportAPI + "/" + probe.Route
		if !wantedVersionContexts[key] || probe.CheckOnly || probe.DeploymentID == "" || probe.DeploymentStatus != "Succeeded" || len(probe.Rows) != 5 {
			t.Fatal("invalid native source-policy capture", key)
		}
		delete(wantedVersionContexts, key)
		checkHash(probe.Metadata, probe.MetadataSHA256)
		digest(probe.DeploymentReceiptSHA256)
		digest(probe.ReceiptSHA256)
		if !strings.Contains(probe.Metadata, "<apiVersion>"+probe.API+"</apiVersion>") {
			t.Fatal("native source-policy component API changed")
		}
		wantedVersionRows := map[string]bool{"K02": true, "K04": true, "K09": true, "U04": true, "U11": true}
		for _, row := range probe.Rows {
			if !wantedVersionRows[row.ID] || row.Native == nil {
				t.Fatal("unexpected, duplicate or incomplete native source-policy row", key, row.ID)
			}
			delete(wantedVersionRows, row.ID)
			versionRows++
			versionCells += roleGroupObservationLeaves(row.Native)
		}
		if len(wantedVersionRows) != 0 {
			t.Fatal("native source-policy row matrix incomplete", key)
		}
	}
	if len(wantedVersionContexts) != 0 || versionRows != data.VersionProbeRowCount || versionCells != data.VersionProbeScalarCells {
		t.Fatalf("native source-policy coverage changed: missing=%d rows=%d cells=%d", len(wantedVersionContexts), versionRows, versionCells)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}

type picklistOrderReplayRow struct {
	id, route string
	native    map[string]any
}

type picklistOrderReplayContext struct {
	sourceAPI, transportAPI, metadata, anonymous, testClass string
	sources                                                 map[string]string
	methods                                                 int
	rows                                                    []picklistOrderReplayRow
}

func TestPicklistOrderNativeConformance(t *testing.T) {
	data := loadPicklistOrderNativeFixture(t)
	for _, capture := range data.Captures {
		context := picklistOrderReplayContext{
			sourceAPI: capture.API, transportAPI: capture.TransportAPI, metadata: capture.Metadata,
			anonymous: data.AnonymousSource, testClass: "OwnedPicklistOrderTest", sources: data.Sources, methods: 25,
		}
		for _, row := range data.Rows {
			if row.API == capture.API {
				context.rows = append(context.rows, picklistOrderReplayRow{id: row.ID, route: row.Route, native: row.Native})
			}
		}
		runPicklistOrderNativeReplay(t, data, context)
	}
}

func TestPicklistOrderSourcePolicyNativeConformance(t *testing.T) {
	data := loadPicklistOrderNativeFixture(t)
	for _, probe := range data.VersionProbes {
		context := picklistOrderReplayContext{
			sourceAPI: probe.API, transportAPI: probe.TransportAPI, metadata: probe.Metadata,
			anonymous: data.VersionAnonymousSource, testClass: "OwnedPicklistVersionTest", sources: data.VersionSources, methods: 5,
		}
		for _, row := range probe.Rows {
			context.rows = append(context.rows, picklistOrderReplayRow{id: row.ID, route: probe.Route, native: row.Native})
		}
		runPicklistOrderNativeReplay(t, data, context)
	}
}

func runPicklistOrderNativeReplay(t *testing.T, data picklistOrderNativeFixture, context picklistOrderReplayContext) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), data.ProjectSource)
	for path, source := range data.MetadataFiles {
		writeFile(t, filepath.Join(root, path), source)
	}
	for name, source := range context.sources {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", context.metadata)
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: []string{context.testClass}})
	if len(cases) != context.methods {
		t.Fatal("native named selection changed")
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	for _, row := range context.rows {
		t.Run(context.sourceAPI+"/"+context.transportAPI+"/"+row.route+"/"+row.id, func(t *testing.T) {
			org := orgFromIndex(index)
			org.APIVersion = context.transportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			marker := "PICKLIST_ORDER|" + row.id + "|"
			payload := ""
			if row.route == "anonymous" {
				source := ""
				for _, line := range strings.Split(context.anonymous, "\n") {
					if strings.Contains(line, marker) {
						if source != "" {
							t.Fatal("ambiguous captured anonymous line")
						}
						source = line
					}
				}
				if source == "" {
					t.Fatal("missing captured anonymous line")
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: context.transportAPI})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, message := range result.Debug {
					if _, value, ok := strings.Cut(message, marker); ok {
						payload = value
					}
				}
			} else if row.route == "istest" {
				found := false
				for _, testCase := range cases {
					if !strings.EqualFold(testCase.MethodName, row.id) {
						continue
					}
					found = true
					key := testCaseKey(testCase)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, execErr := machine.ExecuteInClass(programs[key], testCase.ClassName)
					var observed *vm.RuntimeError
					if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
						t.Fatalf("missing observation assertion: %v", execErr)
					}
					_, payload, _ = strings.Cut(observed.ExceptionMessage(), marker)
				}
				if !found {
					t.Fatal("missing captured named method")
				}
			} else {
				t.Fatal("uncaptured route", row.route)
			}
			var actual map[string]any
			if payload == "" || json.Unmarshal([]byte(payload), &actual) != nil {
				t.Fatal("invalid observation frame")
			}
			if !reflect.DeepEqual(row.native, actual) {
				t.Fatalf("native=%v local=%v", row.native, actual)
			}
		})
	}
}
