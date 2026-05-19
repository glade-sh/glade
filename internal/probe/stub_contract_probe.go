package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/open-aer/oaer/internal/capability"
)

var (
	stubContractProbeOnce sync.Once
	stubContractProbeByID map[string]capability.StubContractProbeSpec
)

func stubContractProbeSpecByID(id string) (capability.StubContractProbeSpec, bool) {
	stubContractProbeOnce.Do(func() {
		stubContractProbeByID = map[string]capability.StubContractProbeSpec{}
		const manifestPath = "docs/generated/STUB_CONTRACT_PROBE_MANIFEST.json"
		data, err := os.ReadFile(manifestPath)
		specs := make([]capability.StubContractProbeSpec, 0)
		if err == nil {
			if err := json.Unmarshal(data, &specs); err != nil {
				specs = nil
			}
		}
		if len(specs) == 0 {
			contracts, buildErr := capability.BuildStubContractReport("example-projects/stubs")
			if buildErr == nil {
				specs = capability.BuildStubContractProbeManifest(contracts, "full")
			}
		}
		for _, spec := range specs {
			stubContractProbeByID[spec.ID] = spec
		}
	})
	spec, ok := stubContractProbeByID[id]
	return spec, ok
}

func isStubContractProbeID(id string) bool {
	return strings.HasPrefix(id, "stub.")
}

type apexCompileError struct {
	Problem string
}

func (e *apexCompileError) Error() string {
	return "apex compile failed: " + strings.TrimSpace(e.Problem)
}

func stubContractCompileFailureResult(spec capability.StubContractProbeSpec, err error) (ProbeResult, bool) {
	var compileErr *apexCompileError
	if !errors.As(err, &compileErr) {
		return ProbeResult{}, false
	}
	exceptionType := "System.CompileException"
	exceptionMessage := strings.TrimSpace(compileErr.Problem)
	return ProbeResult{
		ProbeID:          spec.ID,
		Category:         "Stub Contracts",
		Result:           nil,
		ExceptionType:    &exceptionType,
		ExceptionMessage: &exceptionMessage,
	}, true
}

func buildLocalStubContractProbeCode(spec capability.StubContractProbeSpec) string {
	invoke := stubContractInvocationCode(spec)
	return fmt.Sprintf(`Map<String, Object> payload = new Map<String, Object>{'probeId' => '%s', 'category' => 'Stub Contracts'};
try {
  Object resultValue = null;
  %s
  payload.put('result', resultValue);
  payload.put('exceptionType', null);
  payload.put('exceptionMessage', null);
} catch (Exception ex) {
  payload.put('result', null);
  payload.put('exceptionType', ex.getTypeName());
  payload.put('exceptionMessage', ex.getMessage());
}
System.debug(JSON.serialize(payload));`, spec.ID, invoke)
}

func buildOrgStubContractProbeCode(spec capability.StubContractProbeSpec) string {
	invoke := stubContractInvocationCode(spec)
	return fmt.Sprintf(`Map<String, Object> payload = new Map<String, Object>{'probeId' => '%s', 'category' => 'Stub Contracts'};
try {
  Object resultValue = null;
  %s
  payload.put('result', resultValue);
  payload.put('exceptionType', null);
  payload.put('exceptionMessage', null);
} catch (Exception ex) {
  payload.put('result', null);
  payload.put('exceptionType', ex.getTypeName());
  payload.put('exceptionMessage', ex.getMessage());
}
System.assert(false, 'OAER_PROBE:' + JSON.serialize(payload));`, spec.ID, invoke)
}

func stubContractInvocationCode(spec capability.StubContractProbeSpec) string {
	typeName := spec.Type
	if strings.TrimSpace(spec.Member) == "" {
		return fmt.Sprintf("resultValue = '%s';", typeName)
	}
	args := make([]string, 0, len(spec.Parameters))
	for _, p := range spec.Parameters {
		args = append(args, defaultApexArgForType(p))
	}
	argText := strings.Join(args, ", ")
	returnType := strings.TrimSpace(spec.ReturnType)
	if returnType == "" {
		returnType = "Object"
	}
	switch spec.Kind {
	case "constructor":
		return fmt.Sprintf("%s oaerObj = new %s(%s); resultValue = oaerObj;", typeName, typeName, argText)
	case "property":
		if spec.Static {
			return fmt.Sprintf("resultValue = %s.%s;", typeName, spec.Member)
		}
		return fmt.Sprintf("%s oaerObj = %s; resultValue = oaerObj.%s;", typeName, defaultApexReceiverForType(typeName), spec.Member)
	default:
		if strings.EqualFold(returnType, "void") {
			if spec.Static {
				return fmt.Sprintf("%s.%s(%s); resultValue = 'void';", typeName, spec.Member, argText)
			}
			return fmt.Sprintf("%s oaerObj = %s; oaerObj.%s(%s); resultValue = 'void';", typeName, defaultApexReceiverForType(typeName), spec.Member, argText)
		}
		if spec.Static {
			return fmt.Sprintf("resultValue = %s.%s(%s);", typeName, spec.Member, argText)
		}
		return fmt.Sprintf("%s oaerObj = %s; resultValue = oaerObj.%s(%s);", typeName, defaultApexReceiverForType(typeName), spec.Member, argText)
	}
}

func defaultApexReceiverForType(typeName string) string {
	trimmed := strings.TrimSpace(typeName)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "blob":
		return "Blob.valueOf('oaer')"
	case "string":
		return "'oaer'"
	case "boolean":
		return "true"
	case "integer", "int", "long", "short", "byte":
		return "1"
	case "double", "decimal":
		return "1.25"
	case "id":
		return "'001000000000001AAA'"
	case "date":
		return "Date.newInstance(2024, 1, 2)"
	case "datetime":
		return "Datetime.newInstance(2024, 1, 2, 3, 4, 5)"
	case "time":
		return "Time.newInstance(3, 4, 5, 0)"
	case "pattern":
		return "Pattern.compile('a+')"
	case "matcher":
		return "Pattern.compile('a+').matcher('aaa')"
	case "jsongenerator":
		return "JSON.createGenerator(false)"
	case "jsonparser":
		return "JSON.createParser('{\"a\":1}')"
	case "type", "system.type":
		return "String.class"
	default:
		return fmt.Sprintf("new %s()", trimmed)
	}
}

func defaultApexArgForType(typeName string) string {
	trimmed := strings.TrimSpace(typeName)
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "string":
		return "'oaer'"
	case lower == "boolean":
		return "true"
	case lower == "integer" || lower == "int" || lower == "long" || lower == "short" || lower == "byte":
		return "1"
	case lower == "double" || lower == "decimal":
		return "1.25"
	case lower == "id":
		return "'001000000000001AAA'"
	case lower == "date":
		return "Date.newInstance(2024, 1, 2)"
	case lower == "datetime":
		return "Datetime.newInstance(2024, 1, 2, 3, 4, 5)"
	case lower == "time":
		return "Time.newInstance(3, 4, 5, 0)"
	case lower == "blob":
		return "Blob.valueOf('oaer')"
	case strings.HasPrefix(lower, "list<"):
		return fmt.Sprintf("new %s()", trimmed)
	case strings.HasPrefix(lower, "set<"):
		return fmt.Sprintf("new %s()", trimmed)
	case strings.HasPrefix(lower, "map<"):
		return fmt.Sprintf("new %s()", trimmed)
	default:
		return "null"
	}
}
