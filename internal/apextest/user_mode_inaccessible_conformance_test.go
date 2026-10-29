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
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type userModeInaccessibleSource struct {
	Anonymous    string            `json:"anonymous"`
	Classes      map[string]string `json:"classes"`
	Tests        []string          `json:"tests"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
}
type userModeInaccessibleCell struct {
	AssertionStatus string                     `json:"assertionStatus"`
	Fixture         string                     `json:"fixture"`
	WireFormat      string                     `json:"wireFormat"`
	Marker          string                     `json:"marker"`
	Source          userModeInaccessibleSource `json:"source"`
	NativeStatus    string                     `json:"nativeStatus"`
	Native          json.RawMessage            `json:"native"`
	Outcome         string                     `json:"outcome"`
	Value           string                     `json:"value"`
	Primary         string                     `json:"primary"`
	ExceptionType   string                     `json:"exceptionType"`
	Message         string                     `json:"message"`
	CarryReason     string                     `json:"carryReason"`
}
type userModeInaccessibleFixture struct {
	Names                map[string]string `json:"names"`
	Metadata             map[string]string `json:"metadata"`
	SourceManifestSHA256 string            `json:"sourceManifestSHA256"`
}

func runUserModeInaccessibleConformance(t *testing.T, cluster string, shapes, asserted, carried int) {
	t.Helper()
	var data struct {
		APIVersions              []string `json:"apiVersions"`
		Cluster                  string   `json:"cluster"`
		ShapeCount               int      `json:"shapeCount"`
		AssertedObservationCount int      `json:"assertedObservationCount"`
		CarriedObservationCount  int      `json:"carriedObservationCount"`
		Rows                     []struct {
			ID     string                                         `json:"id"`
			Routes map[string]map[string]userModeInaccessibleCell `json:"routes"`
		} `json:"rows"`
	}
	userModeInaccessibleRead(t, cluster+".json", &data)
	var fixtures map[string]userModeInaccessibleFixture
	userModeInaccessibleRead(t, "fixtures.json", &fixtures)
	fixtureSets := map[string]map[string]userModeInaccessibleFixture{"": fixtures}
	for _, name := range []string{"retry", "boundary", "bracket"} {
		var extra map[string]userModeInaccessibleFixture
		userModeInaccessibleRead(t, name+"_fixtures.json", &extra)
		fixtureSets[name] = extra
	}
	routes := []string{"anonymous", "istest"}
	apis := "59.0,62.0,67.0,68.0"
	if cluster == "runas_boundary" {
		routes = []string{"istest"} // Native boundary capture is named-route only.
		apis = "59.0,62.0,63.0,64.0,65.0,66.0,67.0,68.0"
	} else if cluster == "api_bracket" {
		routes = []string{"istest"} // Native bracket capture is named-route only.
		apis = "59.0,60.0,61.0,62.0,63.0,64.0,65.0,66.0,67.0"
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.AssertedObservationCount != asserted || data.CarriedObservationCount != carried || strings.Join(data.APIVersions, ",") != apis {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	actual, carries := 0, 0
	for _, row := range data.Rows {
		validID := strings.HasPrefix(row.ID, "C185ID_")
		if cluster == "runas_boundary" {
			validID = strings.HasPrefix(row.ID, "C185BC_") || strings.HasPrefix(row.ID, "C185BD_")
		} else if cluster == "api_bracket" {
			validID = strings.HasPrefix(row.ID, "C138_")
		}
		if !validID || seen[row.ID] || len(row.Routes) != len(routes) {
			t.Fatal("invalid row/route matrix")
		}
		seen[row.ID] = true
		for _, route := range routes {
			if len(row.Routes[route]) != len(data.APIVersions) {
				t.Fatal("missing API cells")
			}
			for _, api := range data.APIVersions {
				cell, ok := row.Routes[route][api]
				if !ok || !json.Valid(cell.Native) {
					t.Fatal("missing native receipt")
				}
				if _, ok := fixtureSets[cell.Fixture][api]; !ok || (cell.WireFormat != "" && cell.WireFormat != "compact-v2" && cell.WireFormat != "bracket-sha256-v1") {
					t.Fatal("missing cell metadata or unknown wire format")
				}
				if cell.WireFormat == "bracket-sha256-v1" {
					if cluster != "api_bracket" || cell.Fixture != "bracket" || !regexp.MustCompile(`^B\|[0-9a-f]{2}\|$`).MatchString(cell.Marker) {
						t.Fatal("invalid bracket transport binding")
					}
				} else if cell.Marker != "" {
					t.Fatal("unexpected observation marker")
				}
				source := cell.Source
				texts := map[string]string{}
				if route == "anonymous" {
					texts["anonymous"] = source.Anonymous
				}
				for name, code := range source.Classes {
					texts["class:"+name] = code
				}
				if len(texts) == 0 || len(texts) != len(source.SourceSHA256) {
					t.Fatal("incomplete captured source hashes")
				}
				for name, code := range texts {
					hash := sha256.Sum256([]byte(code))
					if code == "" || hex.EncodeToString(hash[:]) != source.SourceSHA256[name] {
						t.Fatalf("captured source changed: %s", name)
					}
				}
				if cell.Outcome == "carry" {
					if cell.CarryReason == "" {
						t.Fatal("invalid carry")
					}
					if cell.NativeStatus == "OBSERVED" {
						// No observed visibility cells remain carried after the
						// standard-object default sharing correction.
						t.Fatal("uncarried native row disabled")
					} else if cell.AssertionStatus != "" {
						t.Fatal("invalid non-answer carry status")
					}
					carries++
					continue
				}
				if cell.NativeStatus != "OBSERVED" || cell.CarryReason != "" || cell.AssertionStatus != "" {
					t.Fatal("non-answer admitted")
				}
				switch cell.Outcome {
				case "runtime":
					if !json.Valid([]byte(cell.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if cell.Primary == "" {
						t.Fatal("empty native compile rejection")
					}
				case "uncaught":
					if cell.ExceptionType == "" || cell.Message == "" {
						t.Fatal("empty native exception")
					}
				default:
					t.Fatal("invalid native outcome")
				}
				actual++
			}
		}
	}
	if actual != asserted || carries != carried {
		t.Fatalf("cells=%d/%d, want=%d/%d", actual, carries, asserted, carried)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "UserModeInaccessible-"+cluster, api)
			// A private shared-runner template for each route preserves the observed
			// anonymous business-data baseline and the test route's empty business data.
			bases := map[string]*conformanceRunner{}
			orgs := map[string]storage.OrgState{}
			used := map[string]bool{}
			for _, row := range data.Rows {
				for _, route := range routes {
					if cell, ok := row.Routes[route][api]; ok {
						used[cell.Fixture] = true
					}
				}
			}
			for name := range used {
				fixture := fixtureSets[name][api]
				if len(fixture.Metadata) < 3 || len(fixture.SourceManifestSHA256) != 64 {
					t.Fatal("missing native metadata")
				}
				baseIndex := userModeInaccessibleIndex(t, api, fixture, nil)
				for _, route := range routes {
					key := name + "/" + route
					org := userModeInaccessibleOrg(t, baseIndex, fixture, route)
					orgs[key] = org
					bases[key] = newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
				}
			}
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range routes {
						t.Run(route, func(t *testing.T) {
							cell := row.Routes[route][api]
							if cell.Outcome == "carry" {
								counts.TrackKind(t, route, "carried", 1)
								t.Logf("carried %s/%s/%s: %s", row.ID, api, route, cell.CarryReason)
								return
							}
							kind := "exact"
							if cell.Outcome == "reject" {
								kind = "category"
							}
							if row.ID == "C185ID_004" || row.ID == "C185BC_004" {
								kind = "partial" // Environment baseline, stable facts only.
							}
							counts.TrackKind(t, route, kind, 1)
							source := cell.Source
							fixture := fixtureSets[cell.Fixture][api]
							baseKey := cell.Fixture + "/" + route
							index := userModeInaccessibleIndex(t, api, fixture, source.Classes)
							org := orgs[baseKey]
							var execErr error
							if route == "anonymous" {
								setup := sema.Analyze(index)
								if index.HasErrors() || setup.HasErrors() {
									t.Fatalf("native setup accepted; Glade=%v %v", index.Diagnostics, setup.Diagnostics)
								}
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								userModeInaccessibleCompile(t, cell, analysis.Diagnostics, compileErr)
								if cell.Outcome == "reject" {
									return
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: bases[baseKey], LinkProject: true, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								userModeInaccessibleCompile(t, cell, diagnostics, nil)
								if cell.Outcome == "reject" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("missing captured test class")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("test discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("test compilation: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: bases[baseKey], LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							userModeInaccessibleValue(t, row.ID, cell, fixture, execErr)
						})
					}
				})
			}
		})
	}
}

func userModeInaccessibleRead(t *testing.T, file string, target any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/user_mode_inaccessible", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}
func userModeInaccessibleIndex(t *testing.T, api string, fixture userModeInaccessibleFixture, classes map[string]string, projects ...*conformanceProjectCache) typesys.Index {
	t.Helper()
	if len(projects) != 0 {
		files := make(map[string]string, len(fixture.Metadata)+2*len(classes))
		for path, source := range fixture.Metadata {
			files[filepath.Join("force-app/main/default", path)] = source
		}
		for name, source := range classes {
			path := filepath.Join("force-app/main/default/classes", name+".cls")
			files[path] = source
			files[path+"-meta.xml"] = "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"
		}
		return projects[0].index(t, api, files)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for path, text := range fixture.Metadata {
		writeFile(t, filepath.Join(root, "force-app/main/default", path), text)
	}
	for name, text := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, text)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}
func userModeInaccessibleOrg(t *testing.T, index typesys.Index, fixture userModeInaccessibleFixture, route string) storage.OrgState {
	t.Helper()
	org := orgFromIndex(index)
	// The capture org's REST endpoint is 68 on every source API.
	org.APIVersion = "68.0"
	// Seed only fixture state recorded in the captured row and metadata.py. Grants
	// come from the exported permission sets, never from field-name heuristics.
	profiles := org.Objects["Profile"]
	for id, record := range profiles.Records {
		if record.Fields["Name"].String == "Minimum Access - Salesforce" {
			record.Fields["PermissionsViewAllData"] = storage.BooleanValue(false)
			profiles.Records[id] = record
		}
	}
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	for id, record := range licenses.Records {
		if record.Fields["Name"].String == "Salesforce" {
			record.Fields["TotalLicenses"] = storage.IntegerValue(2)
			record.Fields["UsedLicenses"] = storage.IntegerValue(1)
			licenses.Records[id] = record
		}
	}
	org.Objects["UserLicense"] = licenses
	var runningID storage.ID
	for id, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["Name"].String == fixture.Names["running"] {
			runningID = id
		}
	}
	if runningID == "" {
		t.Fatal("missing running-user permission set")
	}
	seed := storage.NewFixture()
	seed.Objects = []storage.FixtureObject{{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue(runningID)}}}}}
	if route == "anonymous" {
		seed.Objects = append(seed.Objects, storage.FixtureObject{Name: "Account", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic baseline account")}}}})
	}
	if err := storage.ApplyFixture(&org, seed); err != nil {
		t.Fatal(err)
	}
	return org
}
func userModeInaccessibleCompile(t *testing.T, cell userModeInaccessibleCell, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	rejected := compileErr != nil
	for _, d := range diagnostics {
		rejected = rejected || d.Severity == diagnostic.Error
	}
	// Category only: the exact native text is evidence, not Glade wording.
	if cell.Outcome == "reject" {
		if cell.Primary == "" || !rejected {
			t.Fatalf("native rejects <%s>; Glade=%v %v", cell.Primary, diagnostics, compileErr)
		}
		return
	}
	if rejected {
		t.Fatalf("native accepts compilation; Glade=%v %v", diagnostics, compileErr)
	}
}
func userModeInaccessibleExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

var userModeInaccessibleRecordID = regexp.MustCompile(`\b[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?\b`)

func userModeInaccessibleNormalize(text string, fixture userModeInaccessibleFixture) string {
	for key, replacement := range map[string]string{"second": "SecondSecret__c", "secret": "Secret__c", "running": "RunningFixtureAccess", "limited": "LimitedFixtureAccess", "object": "BoundaryDenied__c", "record": "BracketRecord__c", "event": "BracketEvent__e"} {
		if name := fixture.Names[key]; name != "" {
			text = strings.ReplaceAll(text, name, replacement)
		}
	}
	return userModeInaccessibleRecordID.ReplaceAllStringFunc(text, func(value string) string {
		if strings.ContainsAny(value, "0123456789") {
			return "<RECORD_ID>"
		}
		return value
	})
}
func userModeInaccessibleValue(t *testing.T, id string, cell userModeInaccessibleCell, fixture userModeInaccessibleFixture, err error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("native %s; Glade missing exception: %v", cell.Outcome, err)
	}
	message := runtimeErr.ExceptionMessage()
	if cell.Outcome == "uncaught" {
		if userModeInaccessibleExceptionType(runtimeErr.Type) != cell.ExceptionType || message != cell.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", cell.ExceptionType, cell.Message, runtimeErr.Type, message)
		}
		return
	}
	if userModeInaccessibleExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing observation envelope: %v", err)
	}
	marker, prefix := "C126_RESULT|"+id+"|", "C126_RESULT|"
	if cell.WireFormat == "bracket-sha256-v1" {
		marker, prefix = cell.Marker, "B|"
	}
	_, envelope, found := strings.Cut(message, marker)
	if !found || strings.Count(message, prefix) != 1 {
		t.Fatalf("missing native marker: %s", message)
	}
	if cell.WireFormat == "bracket-sha256-v1" {
		// Verify original bytes before applying fixture-name normalization.
		var wire struct {
			Length int    `json:"n"`
			Hash   string `json:"h"`
			Value  string `json:"v"`
		}
		if err := json.Unmarshal([]byte(envelope), &wire); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(wire.Value))
		if wire.Value == "" || wire.Length != len(wire.Value) || wire.Hash != hex.EncodeToString(hash[:]) {
			t.Fatal("invalid bracket payload length/hash")
		}
		envelope = wire.Value
	}
	var got, want map[string]any
	if err := json.Unmarshal([]byte(userModeInaccessibleNormalize(envelope, fixture)), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(cell.Value), &want); err != nil {
		t.Fatal(err)
	}
	if cell.WireFormat == "compact-v2" {
		got = userModeInaccessibleExpandPayload(got)
	} else if cell.WireFormat == "bracket-sha256-v1" && id != "C138_BASELINE" {
		keys := map[string]string{"es": "effectStarted", "s": "success", "c": "count", "o": "outside", "t": "type", "m": "message", "q": "queryException", "f": "fields", "n": "mapNull"}
		expanded := make(map[string]any, len(got))
		for key, value := range got {
			if name, ok := keys[key]; ok {
				key = name
			}
			expanded[key] = value
		}
		got = expanded
	}
	// The capture normalizer sorts Set members. Assert exact membership and
	// casing without introducing an iteration-order contract for Apex Sets.
	if inaccessible, ok := got["inaccessibleFields"].(map[string]any); ok {
		for _, value := range inaccessible {
			if fields, ok := value.([]any); ok {
				sort.Slice(fields, func(i, j int) bool { return fields[i].(string) < fields[j].(string) })
			}
		}
	}
	if et, ok := got["exceptionType"].(string); ok {
		got["exceptionType"] = userModeInaccessibleExceptionType(et)
	}
	if cell.WireFormat == "bracket-sha256-v1" {
		if et, ok := got["type"].(string); ok {
			got["type"] = userModeInaccessibleExceptionType(et)
		}
	}
	if id == "C185ID_004" || id == "C185BC_004" {
		// Preserve the full native payload in testdata. Capture-org counts,
		// record identities and REST URLs are not portable runtime behavior.
		userModeInaccessibleBaselineFacts(t, "native", want, id == "C185BC_004")
		userModeInaccessibleBaselineFacts(t, "Glade", got, id == "C185BC_004")
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native <%s>; Glade <%s>", cell.Value, envelope)
	}
}

// The captured row is an environment baseline, stable facts only. Missing or wrong
// permission flags and missing fixture prerequisites remain assertion failures.
func userModeInaccessibleBaselineFacts(t *testing.T, origin string, payload map[string]any, boundary bool) {
	t.Helper()
	var differences []string
	fields := []string{"accountRead", "opportunityRead", "secondRead", "secretRead"}
	if boundary {
		fields = []string{"accountRead", "opportunityRead", "customRead"}
	}
	for _, field := range fields {
		if payload[field] != true {
			differences = append(differences, fmt.Sprintf("%s=%v, want true", field, payload[field]))
		}
	}
	profiles, _ := payload["minimalProfiles"].([]any)
	profileFound := false
	for _, value := range profiles {
		profile, ok := value.(map[string]any)
		if !ok || profile["Name"] != "Minimum Access - Salesforce" {
			continue
		}
		profileFound = true
		for _, field := range []string{"PermissionsModifyAllData", "PermissionsViewAllData"} {
			if profile[field] != false {
				differences = append(differences, fmt.Sprintf("Minimum Access - Salesforce.%s=%v, want false", field, profile[field]))
			}
		}
	}
	if !profileFound {
		differences = append(differences, "minimalProfiles: missing profile named Minimum Access - Salesforce")
	}
	licenses, _ := payload["salesforceLicenses"].([]any)
	licenseFound := false
	for _, value := range licenses {
		license, ok := value.(map[string]any)
		if ok && license["Name"] == "Salesforce" {
			licenseFound = true
		}
	}
	if !licenseFound {
		differences = append(differences, "salesforceLicenses: missing license named Salesforce")
	}
	if len(differences) != 0 {
		t.Fatalf("%s C185ID_004 stable facts differ: %s", origin, strings.Join(differences, "; "))
	}
}

// The retry's compact transport renames harness keys only. Object and field
// names inside the inaccessible map are native evidence and stay untouched.
func userModeInaccessibleExpandPayload(payload map[string]any) map[string]any {
	names := map[string]string{
		"et": "exceptionType", "em": "exceptionMessage", "ml": "messageLength", "mt": "messageTruncated",
		"if": "inaccessibleFields", "mn": "mapIsNull", "ms": "mapSize", "mk": "mapKeys", "ns": "nestedSets",
		"sn": "setIsNull", "n": "size", "p": "permissions", "ar": "accountRead", "sr": "secretRead",
		"dr": "secondRead", "opr": "opportunityRead", "nr": "nameRead", "qs": "querySucceeded",
		"es": "effectStarted", "sc": "seedCount", "rc": "resultCount", "os": "outOfScope",
		"sl": "salesforceLicenses", "mp": "minimalProfiles", "ac": "accounts", "oc": "opportunities",
	}
	expand := func(input map[string]any) map[string]any {
		out := make(map[string]any, len(input))
		for key, value := range input {
			if name, ok := names[key]; ok {
				key = name
			}
			out[key] = value
		}
		return out
	}
	out := expand(payload)
	if permissions, ok := out["permissions"].(map[string]any); ok {
		out["permissions"] = expand(permissions)
	}
	if nested, ok := out["nestedSets"].(map[string]any); ok {
		for object, value := range nested {
			if entry, ok := value.(map[string]any); ok {
				nested[object] = expand(entry)
			}
		}
	}
	return out
}

func TestUserModeInaccessibleControlConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "control", 3, 24, 0)
}
func TestUserModeInaccessibleBaselineConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "baseline", 1, 8, 0)
}
func TestUserModeInaccessibleLimitedFieldConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "limited_field", 6, 48, 0)
}
func TestUserModeInaccessibleLimitedFieldSetConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "limited_field_set", 6, 48, 0)
}
func TestUserModeInaccessibleLimitedObjectConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "limited_object", 6, 48, 0)
}
func TestUserModeInaccessibleLimitedAccessibleConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "limited_accessible", 6, 48, 0)
}
func TestUserModeInaccessibleRunningFieldConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "running_field", 6, 48, 0)
}
func TestUserModeInaccessibleRunningObjectConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "running_object", 6, 48, 0)
}
func TestUserModeInaccessibleUnrelatedQueryExceptionConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "unrelated_query_exception", 1, 8, 0)
}

func TestUserModeInaccessibleRunAsBoundaryConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "runas_boundary", 13, 104, 0)
}

func TestUserModeInaccessibleAPIBracketConformance(t *testing.T) {
	runUserModeInaccessibleConformance(t, "api_bracket", 6, 54, 0)
}
