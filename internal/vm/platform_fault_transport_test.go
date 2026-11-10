package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// The platform-fault classifier is transport-local and must not change the
// serialized RuntimeError shape used by existing consumers.
func TestPlatformFaultDoesNotChangeSerializedRuntimeError(t *testing.T) {
	err := NewUncatchablePlatformFault("synthetic failure")
	body, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(body), "PlatformFault") {
		t.Fatalf("runtime-only marker escaped serialization: %s", body)
	}
	if !strings.Contains(string(body), `"Type":"RuntimeError"`) || !strings.Contains(string(body), `"Message":"synthetic failure"`) {
		t.Fatalf("runtime error shape changed: %s", body)
	}
}
