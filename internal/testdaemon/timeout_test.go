package testdaemon

import (
	"strings"
	"testing"
)

func TestTestTimeoutSourceValidation(t *testing.T) {
	for _, source := range []string{"", "flag", "environment", "default"} {
		if err := validateRunRequestV1(RunRequestV1{TimeoutMS: 0, TimeoutSource: source, Parallelism: 1}); err != nil {
			t.Fatalf("zero cap with source %q rejected: %v", source, err)
		}
	}
	err := validateRunRequestV1(RunRequestV1{TimeoutMS: 1, TimeoutSource: "unknown", Parallelism: 1})
	if err == nil || !strings.Contains(err.Error(), "timeout source") {
		t.Fatalf("unknown timeout source accepted or unclear error: %v", err)
	}
}
