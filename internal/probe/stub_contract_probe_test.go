package probe

import (
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/capability"
)

func TestStubContractInvocationCodeStaticMethod(t *testing.T) {
	spec := capability.StubContractProbeSpec{
		ID:         "stub.string.valueof",
		Type:       "String",
		Member:     "valueOf",
		Kind:       "method",
		Static:     true,
		ReturnType: "String",
		Parameters: []string{"Object"},
	}
	code := stubContractInvocationCode(spec)
	if !strings.Contains(code, "String.valueOf(") {
		t.Fatalf("invocation missing static call: %q", code)
	}
}

func TestStubContractInvocationCodeInstanceProperty(t *testing.T) {
	spec := capability.StubContractProbeSpec{
		ID:     "stub.address.city",
		Type:   "Address",
		Member: "city",
		Kind:   "property",
	}
	code := stubContractInvocationCode(spec)
	if !strings.Contains(code, "new Address()") || !strings.Contains(code, ".city") {
		t.Fatalf("unexpected property invocation: %q", code)
	}
}

func TestStubContractInvocationCodeValueTypeReceiver(t *testing.T) {
	spec := capability.StubContractProbeSpec{
		ID:         "stub.blob.equals",
		Type:       "Blob",
		Member:     "equals",
		Kind:       "method",
		ReturnType: "Boolean",
		Parameters: []string{"Object"},
	}
	code := stubContractInvocationCode(spec)
	if strings.Contains(code, "new Blob()") || !strings.Contains(code, "Blob.valueOf('oaer')") {
		t.Fatalf("unexpected Blob receiver invocation: %q", code)
	}
}

func TestStubContractCompileFailureResult(t *testing.T) {
	spec := capability.StubContractProbeSpec{ID: "stub.missing.type", Type: "MissingType"}
	result, ok := stubContractCompileFailureResult(spec, &apexCompileError{Problem: "Type is not visible: MissingType"})
	if !ok {
		t.Fatalf("expected compile failure result")
	}
	if result.ProbeID != spec.ID || result.ExceptionType == nil || *result.ExceptionType != "System.CompileException" {
		t.Fatalf("unexpected compile failure result: %#v", result)
	}
}

func TestDefaultApexArgForType(t *testing.T) {
	tests := map[string]string{
		"String":       "'oaer'",
		"Integer":      "1",
		"Boolean":      "true",
		"List<String>": "new List<String>()",
		"CustomType":   "null",
	}
	for in, want := range tests {
		if got := defaultApexArgForType(in); got != want {
			t.Fatalf("%s arg = %q, want %q", in, got, want)
		}
	}
}

func TestStubContractInvocationCodeFactoryReceivers(t *testing.T) {
	tests := []struct {
		name     string
		spec     capability.StubContractProbeSpec
		contains string
	}{
		{
			name: "matcher receiver",
			spec: capability.StubContractProbeSpec{
				ID: "stub.matcher.find", Type: "Matcher", Member: "find", Kind: "method", ReturnType: "Boolean",
			},
			contains: "Pattern.compile('a+').matcher('aaa')",
		},
		{
			name: "pattern receiver",
			spec: capability.StubContractProbeSpec{
				ID: "stub.pattern.matcher.sig-string", Type: "Pattern", Member: "matcher", Kind: "method", ReturnType: "Matcher", Parameters: []string{"String"},
			},
			contains: "Pattern.compile('a+')",
		},
		{
			name: "json generator receiver",
			spec: capability.StubContractProbeSpec{
				ID: "stub.jsongenerator.getasstring", Type: "JSONGenerator", Member: "getAsString", Kind: "method", ReturnType: "String",
			},
			contains: "JSON.createGenerator(false)",
		},
		{
			name: "json parser receiver",
			spec: capability.StubContractProbeSpec{
				ID: "stub.jsonparser.getcurrenttoken", Type: "JSONParser", Member: "getCurrentToken", Kind: "method", ReturnType: "JSONToken",
			},
			contains: "JSON.createParser('{\"a\":1}')",
		},
	}
	for _, tc := range tests {
		code := stubContractInvocationCode(tc.spec)
		if !strings.Contains(code, tc.contains) {
			t.Fatalf("%s invocation missing receiver %q: %q", tc.name, tc.contains, code)
		}
	}
}
