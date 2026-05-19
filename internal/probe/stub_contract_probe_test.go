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
