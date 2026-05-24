package oracle

import (
	"strings"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
)

func LocalRunsFromTestReport(project string, report testreport.Run) []OracleRun {
	runs := make([]OracleRun, 0)
	for _, suite := range report.Suites {
		for _, testCase := range suite.Cases {
			className := testCase.ClassName
			if className == "" {
				className = suite.Name
			}
			run := OracleRun{
				SchemaVersion: SchemaVersion,
				Source:        "glade",
				Project:       project,
				TestClass:     className,
				TestMethod:    testCase.MethodName,
				Status:        statusFromTestReport(testCase.Status),
				DurationMS:    testCase.DurationMS,
				Events:        oracleEventsFromTrace(testCase.Trace),
			}
			if testCase.Problem != nil {
				run.Exception = &OracleException{
					Type:    testCase.Problem.Type,
					Message: testCase.Problem.Message,
					Stack:   testCase.Problem.Detail,
				}
				for _, frame := range testCase.Problem.Stack {
					run.Stack = append(run.Stack, OracleStackFrame{
						Symbol: frame.Symbol,
						File:   frame.File,
						Line:   frame.Line,
						Column: frame.Column,
					})
				}
			}
			runs = append(runs, NormalizeRun(run))
		}
	}
	return runs
}

func statusFromTestReport(status testreport.Status) OracleStatus {
	switch status {
	case "", testreport.StatusPass:
		return OracleStatusPass
	case testreport.StatusFail:
		return OracleStatusFail
	case testreport.StatusSkipped:
		return OracleStatusSkipped
	case testreport.StatusCompileError:
		return OracleStatusCompileError
	case testreport.StatusUnsupported:
		return OracleStatusUnsupported
	case testreport.StatusRuntimeError:
		return OracleStatusRuntimeError
	default:
		return OracleStatusRuntimeError
	}
}

func oracleEventsFromTrace(events []trace.Event) []OracleEvent {
	out := make([]OracleEvent, 0, len(events))
	for i, event := range events {
		out = append(out, NormalizeEvent(OracleEvent{
			Type:     oracleEventTypeFromTrace(event),
			Sequence: i + 1,
			Name:     event.Name,
			Payload:  event.Args,
		}))
	}
	return out
}

func oracleEventTypeFromTrace(event trace.Event) OracleEventType {
	text := strings.ToLower(event.Category + " " + event.Name)
	switch {
	case strings.Contains(text, "soql"):
		return OracleEventSOQL
	case strings.Contains(text, "dml"):
		return OracleEventDML
	case strings.Contains(text, "trigger"):
		return OracleEventTrigger
	case strings.Contains(text, "flow"):
		return OracleEventFlow
	case strings.Contains(text, "workflow"):
		return OracleEventWorkflow
	case strings.Contains(text, "email"):
		return OracleEventEmail
	case strings.Contains(text, "file"), strings.Contains(text, "content"):
		return OracleEventFile
	case strings.Contains(text, "async"):
		return OracleEventAsync
	case strings.Contains(text, "limit"):
		return OracleEventLimit
	case strings.Contains(text, "unsupported"):
		return OracleEventUnsupported
	case strings.Contains(text, "exception"):
		return OracleEventException
	default:
		return OracleEventDebug
	}
}
