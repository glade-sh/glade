package apextest_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestC11ActorGateCarryRejectsOtherFailures(t *testing.T) {
	cases := []struct {
		arm             string
		status          testreport.Status
		method, message string
	}{
		{"restricted", testreport.StatusFail, "target", "Assertion Failed: gate actor ViewAllData: Expected: false, Actual: null"},
		{"admin", testreport.StatusRuntimeError, "baseline", "Assertion Failed: gate admin profile ViewAllData: Expected: true, Actual: false"},
		{"restricted", testreport.StatusFail, "target", "Assertion Failed: gate no supplemental ModifyAllData: Expected: false, Actual: null"},
		{"admin", testreport.StatusFail, "baseline", "Assertion Failed: gate no supplemental ModifyAllData: Expected: false, Actual: null"},
	}
	for _, c := range cases {
		t.Run(c.arm, func(t *testing.T) {
			result := testreport.Case{ClassName: "Synthetic", MethodName: c.method, Status: c.status, Problem: &testreport.Problem{Type: "System.AssertException", Message: c.message}}
			if !c11ActorGateCarry(t, result, "Synthetic", c.method, c.arm) {
				t.Fatal("documented gate was not recognized")
			}
			if c11ActorGateCarry(t, result, "Other", c.method, c.arm) || c11ActorGateCarry(t, result, "Synthetic", "other", c.arm) {
				t.Fatal("different selection was accepted")
			}
			result.Problem.Message = "Assertion Failed: another prerequisite"
			if c11ActorGateCarry(t, result, "Synthetic", c.method, c.arm) {
				t.Fatal("unrelated assertion was accepted")
			}
			result.Problem.Message = c.message
			result.Problem.Type = "System.QueryException"
			if c11ActorGateCarry(t, result, "Synthetic", c.method, c.arm) {
				t.Fatal("unrelated exception was accepted")
			}
			result.Problem.Type = "System.AssertException"
			result.Status = testreport.StatusPass
			if c11ActorGateCarry(t, result, "Synthetic", c.method, c.arm) {
				t.Fatal("wrong status was accepted")
			}
		})
	}
}
