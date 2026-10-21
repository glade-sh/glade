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
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type fieldKeyExpected struct {
	Outcome string          `json:"outcome"`
	Value   json.RawMessage `json:"value"`
	Primary string          `json:"primary"`
	Native  json.RawMessage `json:"native"`
	Carry   string          `json:"carry"`
}

type fieldKeySource struct {
	Anonymous     string                      `json:"anonymous"`
	Classes       map[string]string           `json:"classes"`
	Tests         []string                    `json:"tests"`
	SourceSHA256  map[string]string           `json:"sourceSha256"`
	ExpectedByAPI map[string]fieldKeyExpected `json:"expectedByAPI"`
}

type fieldKeyRow struct {
	ID         string                    `json:"id"`
	Group      string                    `json:"group"`
	Comparison string                    `json:"comparison"`
	Routes     map[string]fieldKeySource `json:"routes"`
}

type fieldKeyMetadata struct {
	Objects        map[string]string `json:"objects"`
	PermissionSets map[string]string `json:"permissionSets"`
	SourceSHA256   map[string]string `json:"sourceSha256"`
}

func TestFieldKeyTransportOrgConformance(t *testing.T) { runFieldKeyConformance(t, "transport", 2) }
func TestFieldKeyDescribeStandardOrgConformance(t *testing.T) {
	runFieldKeyConformance(t, "describe_standard", 18)
}
func TestFieldKeyDescribeCustomOrgConformance(t *testing.T) {
	runFieldKeyConformance(t, "describe_custom", 18)
}
func TestFieldKeyDatabaseOrgConformance(t *testing.T)   { runFieldKeyConformance(t, "database", 36) }
func TestFieldKeyStatementOrgConformance(t *testing.T)  { runFieldKeyConformance(t, "statement", 36) }
func TestFieldKeyKeysOrgConformance(t *testing.T)       { runFieldKeyConformance(t, "keys", 8) }
func TestFieldKeyPopulatedOrgConformance(t *testing.T)  { runFieldKeyConformance(t, "populated", 6) }
func TestFieldKeyNullOrgConformance(t *testing.T)       { runFieldKeyConformance(t, "null", 5) }
func TestFieldKeyRejectionsOrgConformance(t *testing.T) { runFieldKeyConformance(t, "rejections", 3) }
func TestFieldKeyAccessOrgConformance(t *testing.T)     { runFieldKeyConformance(t, "access", 1) }

func fieldKeyRead(t *testing.T, name string, destination any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/field_key_upsert", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		t.Fatal(err)
	}
}

// Routes and API cells stay independent. The six baseline cells retain native
// org counts/licenses but assert only transport structure. Uncaught/artifact
// cells, if present, retain their evidence and never count as semantic matches.
func runFieldKeyConformance(t *testing.T, cluster string, shapes int) {
	t.Helper()
	var data struct {
		Cluster                string        `json:"cluster"`
		APIVersions            []string      `json:"apiVersions"`
		Routes                 []string      `json:"routes"`
		ShapeCount             int           `json:"shapeCount"`
		NativeObservationCount int           `json:"nativeObservationCount"`
		Rows                   []fieldKeyRow `json:"rows"`
	}
	fieldKeyRead(t, cluster+".json", &data)
	if data.Cluster != cluster || len(data.Rows) != shapes || data.ShapeCount != shapes || data.NativeObservationCount != shapes*6 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatal("incomplete captured field-key matrix")
	}
	var metadata fieldKeyMetadata
	fieldKeyRead(t, "metadata.json", &metadata)
	metadataSources := make(map[string]string)
	for name, source := range metadata.Objects {
		metadataSources["object:"+name] = source
	}
	for name, source := range metadata.PermissionSets {
		metadataSources["permissionSet:"+name] = source
	}
	fieldKeyCheckSources(t, metadataSources, metadata.SourceSHA256)
	seen := make(map[string]bool)
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 || (row.Comparison != "exact" && (row.Comparison != "transportOnly" || row.Group != "BASELINE")) {
			t.Fatalf("invalid/repeated field-key row %s", row.ID)
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			source, ok := row.Routes[route]
			if !ok || len(source.ExpectedByAPI) != len(data.APIVersions) {
				t.Fatal("missing independent native cells")
			}
			texts := make(map[string]string)
			if source.Anonymous != "" {
				texts["anonymous"] = source.Anonymous
			}
			for name, text := range source.Classes {
				texts["class:"+name] = text
			}
			fieldKeyCheckSources(t, texts, source.SourceSHA256)
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native observation")
				}
				if want.Carry != "" {
					continue
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid(want.Value) {
						t.Fatal("invalid native runtime payload")
					}
				case "compile-rejected":
					if want.Primary == "" {
						t.Fatal("missing exact primary native diagnostic")
					}
				default:
					t.Fatal("unassertable native outcome lacks a carry")
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FieldKey"+cluster, api)
			baseIndex := fieldKeyIndex(t, api, nil, metadata)
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						counts.RunKind(t, route, row.ID, route, fieldKeyCountKind(row, row.Routes[route].ExpectedByAPI[api]), func(t *testing.T) {
							source := row.Routes[route]
							want := source.ExpectedByAPI[api]
							if want.Carry != "" {
								t.Log(want.Carry)
								return
							}
							index := baseIndex
							if route == "isTest" {
								index = fieldKeyIndex(t, api, source.Classes, metadata)
							}
							var execErr error
							var fixturePrefix string
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								items := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								fieldKeyCompile(t, want, items, compileErr)
								if want.Outcome == "compile-rejected" {
									return
								}
								org := fieldKeyOrg(t, index, api)
								fixturePrefix = org.Objects["KeyFixture__c"].Definition.KeyPrefix
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								analysis := sema.Analyze(index)
								items := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								fieldKeyCompile(t, want, items, nil)
								if want.Outcome == "compile-rejected" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("one captured named test class required")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("captured named observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethods(cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								org := fieldKeyOrg(t, index, api)
								fixturePrefix = org.Objects["KeyFixture__c"].Definition.KeyPrefix
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							fieldKeyCompare(t, row, want, execErr, fixturePrefix)
						})
					}
				})
			}
		})
	}
}

func fieldKeyCountKind(row fieldKeyRow, want fieldKeyExpected) string {
	if want.Carry != "" {
		return "carried"
	}
	if row.Comparison == "transportOnly" {
		return "partial"
	}
	return "exact"
}

func fieldKeyCheckSources(t *testing.T, texts, hashes map[string]string) {
	t.Helper()
	if len(texts) == 0 || len(texts) != len(hashes) {
		t.Fatal("incomplete source hash matrix")
	}
	for key, source := range texts {
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != hashes[key] {
			t.Fatalf("captured source changed: %s", key)
		}
	}
}

func fieldKeyIndex(t *testing.T, api string, classes map[string]string, metadata fieldKeyMetadata) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	for name, source := range metadata.Objects {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), source)
	}
	for name, source := range metadata.PermissionSets {
		writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets", name+".permissionset-meta.xml"), source)
	}
	return loadTestIndex(t, root)
}

func fieldKeyOrg(t *testing.T, index typesys.Index, api string) storage.OrgState {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = api
	// The native prerequisite assigns the fixture permission set before row
	// execution. Represent that assignment as setup data, not row DML.
	var permissionID storage.ID
	for id, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["Name"].String == "KeyFixtureAccess" {
			permissionID = id
			break
		}
	}
	userID := storage.ID(storage.DefaultUserID)
	if _, ok := org.Objects["User"].Records[userID]; !ok || permissionID == "" {
		t.Fatal("missing local fixture user/permission set")
	}
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{
		"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(permissionID),
	}}}}}
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	return org
}

func fieldKeyCompile(t *testing.T, want fieldKeyExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var messages []string
	for _, item := range diagnostics {
		if item.Severity != diagnostic.Error {
			continue
		}
		message := item.NativeMessage
		if message == "" {
			message = item.Message
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 && compileErr != nil {
		message := compileErr.Error()
		var syntax *vm.ApexSyntaxError
		if errors.As(compileErr, &syntax) && syntax.NativeMessage != "" {
			message = syntax.NativeMessage
		}
		messages = append(messages, message)
	}
	if want.Outcome == "compile-rejected" {
		if len(messages) == 0 || messages[0] != want.Primary {
			t.Fatalf("native compile rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func fieldKeyCompare(t *testing.T, row fieldKeyRow, want fieldKeyExpected, execErr error, fixturePrefix string) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || (runtimeErr.Type != "System.AssertException" && runtimeErr.Type != "AssertException") {
		t.Fatalf("captured marker assertion missing: %v", execErr)
	}
	marker := "C126_RESULT|" + row.ID + "|"
	message := runtimeErr.ExceptionMessage()
	_, observed, ok := strings.Cut(message, marker)
	if !ok || strings.Count(message, "C126_RESULT|") != 1 {
		t.Fatalf("invalid marker framing: %s", message)
	}
	var wire map[string]any
	if err := json.Unmarshal([]byte(observed), &wire); err != nil {
		t.Fatalf("invalid marker JSON: %v", err)
	}
	actual := fieldKeyExpand(t, row.Group, wire)
	if row.Comparison == "transportOnly" {
		// Counts, active stages and license capacities describe the capture
		// org. Retain them in evidence without asserting those values locally.
		for _, key := range []string{"accounts", "contacts", "opportunities", "users", "fixtureRecords", "stage", "chosenKeys", "salesforceLicenses"} {
			if _, ok := actual[key]; !ok {
				t.Fatalf("missing baseline transport member %s: %#v", key, actual)
			}
		}
		return
	}
	var expected any
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	actualValue := fieldKeyNormalize(actual)
	if row.ID == "C183ID_128" {
		if len(fixturePrefix) != 3 {
			t.Fatal("missing field-key fixture identity")
		}
		fieldKeyBindStoredDuplicateIDs(actualValue, fixturePrefix)
	}
	if !reflect.DeepEqual(actualValue, expected) {
		got, _ := json.Marshal(actualValue)
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, got)
	}
}

// Row128 retains native ID prefixes, but custom prefixes are org allocations.
// Bind only this owned fixture's two IDs in the captured duplicate error.
// Wrong types/prefixes, status, fields, count, label and punctuation still fail
// the exact comparison; no generic message or runtime allocator is changed.
func fieldKeyBindStoredDuplicateIDs(value any, fixturePrefix string) {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			fieldKeyBindStoredDuplicateIDs(item, fixturePrefix)
		}
	case map[string]any:
		fields, ok := value["fields"].([]any)
		if ok && len(fields) == 1 && fields[0] == "ExtText__c" && value["statusCode"] == "DUPLICATE_EXTERNAL_ID" {
			frame := "KeyFixture ExtText: more than one record found for external id field: ["
			id := "<ID:" + fixturePrefix + ">"
			if value["message"] == frame+id+", "+id+"]" {
				value["message"] = frame + "<ID:a00>, <ID:a00>]"
			}
		}
		for _, item := range value {
			fieldKeyBindStoredDuplicateIDs(item, fixturePrefix)
		}
	}
}

// Decode only the capture's v=1 schema. Error references retain multiplicity,
// record indices and exact messages; anonymous composed messages stay asserted.
func fieldKeyExpand(t *testing.T, group string, wire map[string]any) map[string]any {
	t.Helper()
	if wire["v"] != float64(1) {
		t.Fatal("unexpected capture wire version")
	}
	delete(wire, "v")
	var catalog []any
	if value, present := wire["d"]; present {
		var ok bool
		catalog, ok = value.([]any)
		if !ok {
			t.Fatal("invalid error catalog")
		}
		for _, entry := range catalog {
			triple, ok := entry.([]any)
			if !ok || len(triple) != 3 {
				t.Fatal("invalid error catalog triple")
			}
		}
	}
	delete(wire, "d")
	if _, bad := wire["captureTransportError"]; bad {
		t.Fatal("capture transport rejected payload")
	}
	array := func(value any) []any {
		items, ok := value.([]any)
		if !ok {
			t.Fatalf("invalid capture array: %#v", value)
		}
		return items
	}
	errorValue := func(reference any, indexed bool) map[string]any {
		var record any
		if indexed {
			pair := array(reference)
			if len(pair) != 2 {
				t.Fatal("invalid indexed error reference")
			}
			index, ok := pair[0].(float64)
			if !ok || index != float64(int(index)) {
				t.Fatal("invalid DML record index")
			}
			record, reference = pair[0], pair[1]
		}
		number, ok := reference.(float64)
		if !ok || number != float64(int(number)) || number < 0 || int(number) >= len(catalog) {
			t.Fatal("invalid error catalog index")
		}
		triple := array(catalog[int(number)])
		if len(triple) != 3 {
			t.Fatal("invalid error catalog triple")
		}
		result := map[string]any{"statusCode": triple[0], "message": triple[1], "fields": triple[2]}
		if indexed {
			result["recordIndex"] = record
		}
		return result
	}
	exception := func(item map[string]any) map[string]any {
		result := make(map[string]any)
		if value, ok := item["t"]; ok {
			result["exceptionType"] = value
		}
		if value, ok := item["x"]; ok {
			result["exceptionMessage"] = value
		}
		if value, ok := item["e"]; ok {
			items := []any{}
			for _, reference := range array(value) {
				items = append(items, errorValue(reference, true))
			}
			result["errors"] = items
		}
		return result
	}
	steps := func(value any) []any {
		items := []any{}
		for _, entry := range array(value) {
			item, ok := entry.(map[string]any)
			if !ok {
				t.Fatal("invalid capture step")
			}
			result := exception(item)
			if phase, ok := item["p"]; ok {
				if phase != float64(0) && phase != float64(1) {
					t.Fatal("invalid capture phase")
				}
				result["phase"] = []string{"first", "second"}[int(phase.(float64))]
			}
			for short, full := range map[string]string{"i": "idPresent", "n": "recordCount", "eq": "idMatchesPrevious"} {
				if value, ok := item[short]; ok {
					result[full] = value
				}
			}
			if value, ok := item["r"]; ok {
				results := []any{}
				for _, entry := range array(value) {
					r, ok := entry.(map[string]any)
					if !ok {
						t.Fatal("invalid capture result")
					}
					failures := []any{}
					for _, reference := range array(r["e"]) {
						failures = append(failures, errorValue(reference, false))
					}
					results = append(results, map[string]any{"success": r["s"], "created": r["c"], "idPresent": r["i"], "errors": failures})
				}
				result["results"] = results
			}
			items = append(items, result)
		}
		return items
	}
	if group == "BASELINE" {
		counts, keys, licenses := array(wire["n"]), array(wire["k"]), array(wire["l"])
		if len(counts) != 5 || len(keys) != 6 {
			t.Fatal("invalid baseline vector")
		}
		result := map[string]any{"stage": wire["s"]}
		for i, key := range []string{"accounts", "contacts", "opportunities", "users", "fixtureRecords"} {
			result[key] = counts[i]
		}
		chosen := make(map[string]any)
		for i, key := range []string{"accountName", "accountNumber", "contactName", "contactLastName", "opportunityName", "federationIdentifier"} {
			chosen[key] = keys[i]
		}
		result["chosenKeys"] = chosen
		licenseValues := []any{}
		for _, entry := range licenses {
			item := array(entry)
			if len(item) != 3 {
				t.Fatal("invalid license vector")
			}
			licenseValues = append(licenseValues, map[string]any{"Name": item[0], "TotalLicenses": item[1], "UsedLicenses": item[2]})
		}
		result["salesforceLicenses"] = licenseValues
		return result
	}
	if group == "CONTROL" || group == "DESCRIBE_STANDARD" || group == "DESCRIBE_CUSTOM" {
		if _, ok := wire["t"]; ok {
			return exception(wire)
		}
		return wire
	}
	result := exception(wire)
	if group == "ACCESS_LEVEL" {
		for short, full := range map[string]string{"u": "AccessLevel.USER_MODE", "y": "AccessLevel.SYSTEM_MODE"} {
			if value, ok := wire[short]; ok {
				result[full] = steps(value)
			}
		}
	} else if value, ok := wire["s"]; ok {
		result["steps"] = steps(value)
	} else if value, ok := wire["r"]; ok {
		result["results"] = steps([]any{map[string]any{"r": value}})[0].(map[string]any)["results"]
	}
	return result
}

var fieldKeyRecordID = regexp.MustCompile(`(?:[0-9][A-Za-z0-9]{2}|a[0-9A-Za-z]{2})[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?`)
var fieldKeyEmail = regexp.MustCompile(`(?i)key\.-?\d+@example\.invalid`)

func fieldKeyWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}

func fieldKeyNormalizeText(text string, pattern *regexp.Regexp, email bool) string {
	var out strings.Builder
	previous := 0
	for _, span := range pattern.FindAllStringIndex(text, -1) {
		start, end := span[0], span[1]
		if start > 0 && (fieldKeyWordByte(text[start-1]) || email && text[start-1] == '.') || end < len(text) && fieldKeyWordByte(text[end]) {
			continue
		}
		out.WriteString(text[previous:start])
		if email {
			out.WriteString("<KEY_EMAIL>")
		} else {
			out.WriteString("<ID:" + text[start:start+3] + ">")
		}
		previous = end
	}
	out.WriteString(text[previous:])
	return out.String()
}

func fieldKeyNormalize(value any) any {
	switch value := value.(type) {
	case string:
		return fieldKeyNormalizeText(fieldKeyNormalizeText(value, fieldKeyEmail, true), fieldKeyRecordID, false)
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			result[i] = fieldKeyNormalize(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			if key == "exceptionType" {
				// Qualify only the captured builtin type name. Never rewrite
				// exceptionMessage, DML messages or status/error text.
				if name, ok := item.(string); ok {
					switch name {
					case "SObjectException", "DmlException", "NullPointerException", "SecurityException", "TypeException":
						item = "System." + name
					}
				}
			}
			result[key] = fieldKeyNormalize(item)
		}
		return result
	default:
		return value
	}
}
