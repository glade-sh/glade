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
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type invocableFlowActionAnswer struct {
	Outcome          string          `json:"outcome"`
	Value            json.RawMessage `json:"value"`
	ExceptionType    string          `json:"exceptionType"`
	ExceptionMessage string          `json:"exceptionMessage"`
	TestFailures     []struct {
		ExceptionType string `json:"exceptionType"`
		Message       string `json:"message"`
	} `json:"testFailures"`
}
type invocableFlowActionSource struct {
	Code                string                               `json:"code"`
	OperationSHA256     string                               `json:"operationSHA256"`
	CaptureSourceSHA256 map[string]string                    `json:"captureSourceSHA256"`
	ExpectedByAPI       map[string]invocableFlowActionAnswer `json:"expectedByAPI"`
	CarriedByAPI        map[string]json.RawMessage           `json:"carriedByAPI"`
}

func runInvocableFlowActionConformance(t *testing.T, cluster string, shapes, cells, carries int) {
	t.Helper()
	var data struct {
		Cluster                 string   `json:"cluster"`
		APIVersions             []string `json:"apiVersions"`
		NativeObservationCount  int      `json:"nativeObservationCount"`
		CarriedObservationCount int      `json:"carriedObservationCount"`
		Rows                    []struct {
			ID     string                               `json:"id"`
			Routes map[string]invocableFlowActionSource `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/invocable_flow_action", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.CarriedObservationCount != carries || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("invalid native matrix")
	}
	seen := map[string]bool{}
	observed, carried := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("invalid row binding")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "istest"} {
			source, ok := row.Routes[route]
			if !ok {
				t.Fatal("missing independent route")
			}
			sum := sha256.Sum256([]byte(source.Code))
			if hex.EncodeToString(sum[:]) != source.OperationSHA256 || len(source.CaptureSourceSHA256) == 0 {
				t.Fatal("changed capture operation")
			}
			if len(source.ExpectedByAPI)+len(source.CarriedByAPI) != 3 {
				t.Fatal("incomplete native cells")
			}
			for _, api := range data.APIVersions {
				want, answered := source.ExpectedByAPI[api]
				receipt, held := source.CarriedByAPI[api]
				if answered == held {
					t.Fatal("ambiguous admission")
				}
				if held {
					if !json.Valid(receipt) {
						t.Fatal("invalid carry")
					}
					carried++
					continue
				}
				if want.Outcome != "runtime" && want.Outcome != "uncaught_anonymous" && want.Outcome != "uncaught_test_failure" {
					t.Fatalf("unsupported native outcome %s", want.Outcome)
				}
				observed++
			}
		}
	}
	if observed != cells || carried != carries {
		t.Fatal("native count changed")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "C187IA-"+cluster, api)
			root := invocableFlowActionRoot(t, api)
			baseIndex := loadTestIndex(t, root)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			if len(org.Metadata.Flows) != 3 {
				t.Fatalf("synthetic flow metadata: %d", len(org.Metadata.Flows))
			}
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "istest"} {
						source := row.Routes[route]
						want, ok := source.ExpectedByAPI[api]
						if !ok {
							t.Logf("uncaptured %s/%s/%s: %s", row.ID, route, api, source.CarriedByAPI[api])
							continue
						}
						kind := "exact"
						if _, partial := invocableFlowActionPartialFaultIndex(row.ID, api); partial {
							kind = "partial"
						}
						counts.RunKind(t, route, row.ID, route, kind, func(t *testing.T) {
							body := invocableFlowActionBody(row.ID, source.Code, api)
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(baseIndex, body, api)
								invocableFlowActionCompile(t, analysis.Diagnostics)
								program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execErr = base.execute(program)
							} else {
								namedRoot := invocableFlowActionRoot(t, api)
								name := "InvocableFlowObserved" + strings.TrimPrefix(row.ID, "C187IA_")
								path := filepath.Join(namedRoot, "force-app/main/default/classes", name+".cls")
								writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void observe(){"+body+"} }")
								writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
								index := loadTestIndex(t, namedRoot)
								analysis := sema.Analyze(index)
								invocableFlowActionCompile(t, append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...))
								cases := Discover(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatal("missing named route")
								}
								methods, methodErrors := compileTestMethods(cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], name)
							}
							invocableFlowActionCompare(t, row.ID, api, want, execErr)
						})
					}
				})
			}
		})
	}
}

func invocableFlowActionRoot(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for _, kind := range []string{"Success", "NoOutput", "Fault"} {
		raw, err := os.ReadFile(filepath.Join("testdata/conformance/invocable_flow_action", strings.TrimSuffix(api, ".0")+"_"+kind+".flow-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		name := "C187A" + strings.TrimSuffix(api, ".0") + "aaaaaaaa" + kind
		writeFile(t, filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml"), string(raw))
	}
	return root
}
func invocableFlowActionBody(id, code, api string) string {
	if id == "C187IA_020" {
		return code
	}
	for _, kind := range []string{"Success", "NoOutput", "Fault", "Unknown"} {
		code = strings.ReplaceAll(code, "FLOW_"+kind, "C187A"+strings.TrimSuffix(api, ".0")+"aaaaaaaa"+kind)
	}
	// Only capture transport changes: the operation and catch payload are retained.
	return "Map<String,Object> payload=new Map<String,Object>();String step='start';try{" + code + "}catch(Exception e){payload.put('exceptionType',e.getTypeName());payload.put('exceptionMessage',e.getMessage());payload.put('exceptionMessageLength',e.getMessage()==null?null:e.getMessage().length());}payload.put('step',step);System.assert(false,'C126_RESULT|" + id + "|'+JSON.serialize(payload));"
}
func invocableFlowActionCompile(t *testing.T, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			t.Fatalf("native accepted compilation: %s", d.Message)
		}
	}
}
func invocableFlowActionCompare(t *testing.T, id, api string, want invocableFlowActionAnswer, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing native outcome: %v", execErr)
	}
	if want.Outcome != "runtime" {
		if want.Outcome == "uncaught_test_failure" {
			if len(want.TestFailures) != 1 {
				t.Fatal("missing raw native test failure")
			}
			want.ExceptionType = want.TestFailures[0].ExceptionType
			want.ExceptionMessage = want.TestFailures[0].Message
		}
		typ := runtimeErr.Type
		if !strings.Contains(typ, ".") {
			typ = "System." + typ
		}
		// Native uncaught exceptionMessage includes the type; do not strip or reconstruct native text.
		if typ != want.ExceptionType || typ+": "+runtimeErr.ExceptionMessage() != want.ExceptionMessage {
			t.Fatalf("native escaped <%s>; Glade <%s: %s>", want.ExceptionMessage, typ, runtimeErr.ExceptionMessage())
		}
		return
	}
	if runtimeErr.Type != "System.AssertException" {
		t.Fatalf("native payload; Glade %v", execErr)
	}
	_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+id+"|")
	if !ok {
		t.Fatalf("missing row marker: %v", execErr)
	}
	var got, expected any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	invocableFlowActionDecodeErrorCodes(t, got)
	invocableFlowActionDecodeErrorCodes(t, expected)
	got = invocableFlowActionNormalize(got)
	expected = invocableFlowActionNormalize(expected)
	if index, partial := invocableFlowActionPartialFaultIndex(id, api); partial {
		invocableFlowActionComparePartialFault(t, index, got, expected)
	}
	if !reflect.DeepEqual(got, expected) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(expected)
		t.Fatalf("native %s; Glade %s", w, g)
	}
}

// Mirror the capture's native_check.decode_error_codes transport conversion.
// Native expectations may already be decoded; only encoded error DTOs change.
// Invalid transport or gack-shaped messages cannot become a normal code match.
var invocableFlowActionDecodedGack = regexp.MustCompile(`(?i)Error\s*Id\b|internal\s+server\s+error|unexpected\s+error|Internal Salesforce Error`)

func invocableFlowActionDecodeErrorCodes(t *testing.T, value any) {
	t.Helper()
	payload, ok := value.(map[string]any)
	if !ok {
		return
	}
	results, _ := payload["results"].([]any)
	for _, item := range results {
		result, _ := item.(map[string]any)
		errors, _ := result["errors"].([]any)
		for _, item := range errors {
			fault, _ := item.(map[string]any)
			encoded, hasEncoded := fault["codeEncoded"]
			encoding, hasEncoding := fault["codeEncoding"]
			if !hasEncoded && !hasEncoding {
				continue
			}
			if _, hasCode := fault["code"]; hasCode || !hasEncoded || encoding != "base64-utf8" {
				t.Fatal("exact error-code transport required")
			}
			var code any
			if encoded != nil {
				text, ok := encoded.(string)
				if !ok || strings.ContainsAny(text, "\r\n") {
					t.Fatal("error-code transport unavailable")
				}
				decoded, err := base64.StdEncoding.DecodeString(text)
				if err != nil || !utf8.Valid(decoded) {
					t.Fatal("error-code transport unavailable")
				}
				code = string(decoded)
			}
			message, ok := fault["message"].(string)
			if code != "UNKNOWN_EXCEPTION" || !ok || invocableFlowActionDecodedGack.MatchString(message) {
				t.Fatal("encoded error is outside the captured normal parameter rejection")
			}
			delete(fault, "codeEncoded")
			delete(fault, "codeEncoding")
			fault["code"] = code
		}
	}
}

// D-b: 009/012/013 carry a plain guide tail at 62 and an org-hosted anchor
// at 67/68, independently on both routes. Only those twelve cells compare the
// stable error prefix; native anchor shape is checked without matching its host.
func invocableFlowActionPartialFaultIndex(id, api string) (int, bool) {
	if api != "67.0" && api != "68.0" {
		return 0, false
	}
	switch id {
	case "C187IA_009", "C187IA_013":
		return 0, true
	case "C187IA_012":
		return 1, true
	default:
		return 0, false
	}
}

func invocableFlowActionComparePartialFault(t *testing.T, index int, got, expected any) {
	t.Helper()
	faultError := func(value any) map[string]any {
		payload, ok := value.(map[string]any)
		if !ok {
			t.Fatal("missing Flow payload")
		}
		results, ok := payload["results"].([]any)
		if !ok || len(results) <= index {
			t.Fatal("missing Flow fault result")
		}
		result, ok := results[index].(map[string]any)
		if !ok {
			t.Fatal("invalid Flow fault result")
		}
		errors, ok := result["errors"].([]any)
		if !ok || len(errors) != 1 {
			t.Fatal("expected one captured Flow fault")
		}
		fault, ok := errors[0].(map[string]any)
		if !ok {
			t.Fatal("invalid Flow fault error")
		}
		return fault
	}
	native, local := faultError(expected), faultError(got)
	const prefix = "This error occurred when the flow tried to create records: REQUIRED_FIELD_MISSING: Required fields are missing: [Name]. You can look up ExceptionCode values in the "
	nativeMessage, ok := native["message"].(string)
	if !ok || !strings.HasPrefix(nativeMessage, prefix) {
		t.Fatal("native Flow fault stable prefix changed")
	}
	link, guide, anchor := strings.Cut(strings.TrimPrefix(nativeMessage, prefix), "'>")
	if !anchor || !strings.HasPrefix(link, "<a href='https://hostSha256:") || guide != "SOAP API Developer Guide</a>." {
		t.Fatal("native API 67/68 Flow fault must carry the scrubbed SOAP guide anchor")
	}
	localMessage, ok := local["message"].(string)
	if !ok || !strings.HasPrefix(localMessage, prefix) {
		t.Fatal("Glade Flow fault stable prefix differs")
	}
	native["message"], local["message"] = prefix, prefix
	delete(native, "messageLength")
	delete(local, "messageLength")
	t.Log("partial Flow fault comparison: exact prefix and remaining fields; native guide anchor checked; org host, tail and messageLength carried")
}

func invocableFlowActionNormalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if key == "Flow__InterviewGuid" {
				out[key] = "<interview-guid>"
			} else {
				out[key] = invocableFlowActionNormalize(item)
			}
		}
		if message, ok := out["exceptionMessage"].(string); ok && strings.HasPrefix(message, "Salesforce System Error ") {
			out["exceptionMessage"] = "Salesforce System Error "
			delete(out, "exceptionMessageLength")
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = invocableFlowActionNormalize(item)
		}
		return out
	case string:
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) == nil {
			switch decoded.(type) {
			case map[string]any, []any:
				return invocableFlowActionNormalize(decoded)
			}
		}
	}
	return value
}

func TestInvocableFlowActionControlsConformance(t *testing.T) {
	runInvocableFlowActionConformance(t, "controls", 3, 18, 0)
}
func TestInvocableFlowActionFlowConformance(t *testing.T) {
	runInvocableFlowActionConformance(t, "flow", 15, 90, 0)
}
func TestInvocableFlowActionStandardConformance(t *testing.T) {
	runInvocableFlowActionConformance(t, "standard", 2, 12, 0)
}
func TestInvocableFlowActionJSONConformance(t *testing.T) {
	runInvocableFlowActionConformance(t, "json", 20, 120, 0)
}
func TestInvocableFlowActionSerializationConformance(t *testing.T) {
	runInvocableFlowActionConformance(t, "serialization", 4, 24, 0)
}
