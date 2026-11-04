package apextest

import (
	"github.com/glade-sh/glade/internal/testreport"
	"testing"
)

func TestC14WarmProblemRejectsMissingCapture(t *testing.T) {
	for _, tc := range []struct {
		name    string
		problem *testreport.Problem
		capture string
	}{
		{"nil", nil, "expected"},
		{"ordinary error", &testreport.Problem{Type: "System.TypeException", Message: "unexpected error"}, "expected"},
		{"wrong marker", &testreport.Problem{Type: "System.AssertException", Message: `CAPTURE|other|{}`}, "expected"},
		{"uncaught is not a wildcard", &testreport.Problem{Type: "System.AssertException", Message: `CAPTURE|other|{}`}, "UNCAUGHT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c14WarmProblem(tc.problem, tc.capture); err == nil {
				t.Fatal("missing or unrelated capture was accepted")
			}
		})
	}
}
