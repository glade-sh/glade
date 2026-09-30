package gladecli

import (
	"github.com/glade-sh/glade/internal/testreport"
	"testing"
)

func TestFlattenTestCasesPreservesTerminalReason(t *testing.T) {
	run := testreport.Run{Suites: []testreport.Suite{{Name: "Packet", Cases: []testreport.Case{{ClassName: "Packet", MethodName: "deadline", Status: testreport.StatusUnsupported, Reason: testreport.ReasonTimeout}, {ClassName: "Packet", MethodName: "pass", Status: testreport.StatusPass}}}}}
	rows := flattenTestCases(run)
	if rows[0]["reason"] != testreport.ReasonTimeout {
		t.Fatalf("CLI discarded terminal cause: %#v", rows[0])
	}
	if _, exists := rows[1]["reason"]; exists {
		t.Fatal("passing case got a reason")
	}
}
