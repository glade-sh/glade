package oracle

import (
	"testing"

	"github.com/open-aer/oaer/internal/testreport"
)

func TestLocalRunsFromTestReportAdaptsCases(t *testing.T) {
	report := testreport.Run{
		Suites: []testreport.Suite{{
			Name: "AccountOracleTest",
			Cases: []testreport.Case{{
				ClassName:  "AccountOracleTest",
				MethodName: "createsRecord",
				Status:     testreport.StatusFail,
				DurationMS: 5,
				Problem:    &testreport.Problem{Type: "System.AssertException", Message: "bad"},
			}},
		}},
	}

	runs := LocalRunsFromTestReport("fixture", report)
	if len(runs) != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if runs[0].Status != OracleStatusFail || runs[0].Exception == nil || runs[0].Exception.Type != "System.AssertException" {
		t.Fatalf("run = %#v", runs[0])
	}
}
