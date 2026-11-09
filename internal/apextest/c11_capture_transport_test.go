package apextest_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestC11CapturePayloadAcceptsLocalAssertionTransport(t *testing.T) {
	result := testreport.Case{ClassName: "Synthetic", MethodName: "capture", Status: testreport.StatusFail, Problem: &testreport.Problem{Type: "System.AssertException", Message: `Assertion Failed: CAPTURE|synthetic|{"value":false}`}}
	got, err := c12CapturePayload(result, "Synthetic", "capture", "synthetic")
	if err != nil || string(got) != `{"value":false}` {
		t.Fatalf("capture payload %q: %v", got, err)
	}
	result.Problem.Message = `Assertion Failed: CAPTURE|other|{"value":false}`
	if _, err = c12CapturePayload(result, "Synthetic", "capture", "synthetic"); err == nil {
		t.Fatal("accepted unrelated marker")
	}
}
