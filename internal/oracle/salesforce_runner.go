package oracle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/probe"
)

type SalesforceRunOptions struct {
	Project     string
	OrgAlias    string
	Filter      string
	WaitMinute  int
	CaptureLogs bool
	LogLimit    int
}

type SalesforceRunner struct {
	CommandRunner CommandRunner
}

type CommandRunner interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error)
}

type ExecCommandRunner struct{}

type ApexLogObservation struct {
	Events        []OracleEvent        `json:"events,omitempty"`
	DebugPayloads []OracleDebugPayload `json:"debugPayloads,omitempty"`
	Limits        []OracleLimit        `json:"limits,omitempty"`
}

type ApexLogRecord struct {
	ID        string `json:"Id"`
	Operation string `json:"Operation"`
	Request   string `json:"Request"`
	StartTime string `json:"StartTime"`
	Raw       string `json:"-"`
}

func (r SalesforceRunner) RunTests(ctx context.Context, opts SalesforceRunOptions) ([]OracleRun, error) {
	if strings.TrimSpace(opts.OrgAlias) == "" {
		return nil, errors.New("target org is required")
	}
	if strings.TrimSpace(opts.Filter) == "" {
		return nil, errors.New("filter is required for Salesforce oracle test runs")
	}
	wait := opts.WaitMinute
	if wait <= 0 {
		wait = 10
	}
	args := []string{
		"apex", "run", "test",
		"--target-org", opts.OrgAlias,
		"--tests", opts.Filter,
		"--wait", strconv.Itoa(wait),
		"--result-format", "json",
	}
	runner := r.CommandRunner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	stdout, stderr, err := runner.Run(ctx, opts.Project, "sf", args...)
	if err != nil {
		if len(stdout) > 0 {
			if runs, parseErr := ParseSalesforceTestResultJSON(opts.Project, opts.OrgAlias, stdout); parseErr == nil {
				return runs, nil
			}
		}
		return nil, fmt.Errorf("sf apex run test failed: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	runs, err := ParseSalesforceTestResultJSON(opts.Project, opts.OrgAlias, stdout)
	if err != nil {
		return nil, err
	}
	if opts.CaptureLogs {
		runs, err = r.attachRecentApexLogs(ctx, runner, opts, runs)
		if err != nil {
			return nil, err
		}
	}
	return runs, nil
}

func (r SalesforceRunner) RunAnonymous(probeDir, orgAlias, id, category, source string) (OracleRun, error) {
	executor := probe.SFDXExecutor{OrgAlias: orgAlias}
	result, err := executor.CaptureToolingSnippet(probeDir, probe.ToolingSnippet{ID: id, Category: category, Source: source})
	if err != nil {
		return OracleRun{}, err
	}
	return OracleRunFromToolingSnippet(probeDir, orgAlias, result), nil
}

func (ExecCommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (r SalesforceRunner) attachRecentApexLogs(ctx context.Context, runner CommandRunner, opts SalesforceRunOptions, runs []OracleRun) ([]OracleRun, error) {
	if opts.LogLimit <= 0 {
		opts.LogLimit = len(runs)*3 + 20
	}
	logs, err := r.fetchRecentApexLogs(ctx, runner, opts)
	if err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return runs, nil
	}
	out := make([]OracleRun, len(runs))
	copy(out, runs)
	for i := range out {
		for _, log := range logs {
			if !apexLogMatchesRun(log.Raw, out[i]) {
				continue
			}
			obs := ParseApexLog(log.Raw)
			out[i].Events = append(out[i].Events, obs.Events...)
			out[i].DebugPayloads = append(out[i].DebugPayloads, obs.DebugPayloads...)
			out[i].Limits = append(out[i].Limits, obs.Limits...)
			out[i].RawArtifacts = append(out[i].RawArtifacts, OracleArtifact{Type: "ApexLog", Path: log.ID, Raw: log.Raw})
		}
		out[i] = NormalizeRun(out[i])
	}
	return out, nil
}

func (r SalesforceRunner) fetchRecentApexLogs(ctx context.Context, runner CommandRunner, opts SalesforceRunOptions) ([]ApexLogRecord, error) {
	limit := opts.LogLimit
	if limit <= 0 {
		limit = 20
	}
	if limit < 20 {
		limit = 20
	}
	query := fmt.Sprintf("SELECT Id, Operation, Request, StartTime FROM ApexLog ORDER BY StartTime DESC LIMIT %d", limit)
	stdout, stderr, err := runner.Run(ctx, opts.Project, "sf", "data", "query", "--use-tooling-api", "--target-org", opts.OrgAlias, "--query", query, "--json")
	if err != nil {
		return nil, fmt.Errorf("sf data query ApexLog failed: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	var output struct {
		Result struct {
			Records []ApexLogRecord `json:"records"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout, &output); err != nil {
		return nil, fmt.Errorf("parse ApexLog query JSON: %w", err)
	}
	logs := make([]ApexLogRecord, 0, len(output.Result.Records))
	for _, record := range output.Result.Records {
		if strings.TrimSpace(record.ID) == "" {
			continue
		}
		raw, fetchStderr, fetchErr := runner.Run(ctx, opts.Project, "sf", "apex", "get", "log", "--target-org", opts.OrgAlias, "--log-id", record.ID)
		if fetchErr != nil {
			return nil, fmt.Errorf("sf apex get log %s failed: %w (stderr: %s)", record.ID, fetchErr, strings.TrimSpace(string(fetchStderr)))
		}
		record.Raw = string(raw)
		logs = append(logs, record)
	}
	return logs, nil
}

func apexLogMatchesRun(raw string, run OracleRun) bool {
	lower := strings.ToLower(raw)
	if strings.TrimSpace(run.TestClass) != "" && !strings.Contains(lower, strings.ToLower(run.TestClass)) {
		return false
	}
	if strings.TrimSpace(run.TestMethod) != "" && !strings.Contains(lower, strings.ToLower(run.TestMethod)) {
		return false
	}
	return true
}

func ParseSalesforceTestResultJSON(project, orgAlias string, raw []byte) ([]OracleRun, error) {
	var output struct {
		Status int `json:"status"`
		Result struct {
			Summary map[string]any `json:"summary"`
			Tests   []struct {
				ApexClass struct {
					Name string `json:"Name"`
				} `json:"ApexClass"`
				ClassName   string `json:"ClassName"`
				MethodName  string `json:"MethodName"`
				Outcome     string `json:"Outcome"`
				Message     string `json:"Message"`
				StackTrace  string `json:"StackTrace"`
				RunTime     int64  `json:"RunTime"`
				QueueItemID string `json:"QueueItemId"`
			} `json:"tests"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("parse Salesforce test result JSON: %w", err)
	}
	runs := make([]OracleRun, 0, len(output.Result.Tests))
	for _, test := range output.Result.Tests {
		className := test.ApexClass.Name
		if className == "" {
			className = test.ClassName
		}
		run := OracleRun{
			SchemaVersion: SchemaVersion,
			Source:        "salesforce",
			Project:       project,
			OrgAlias:      orgAlias,
			TestClass:     className,
			TestMethod:    test.MethodName,
			Status:        statusFromSalesforceOutcome(test.Outcome),
			DurationMS:    test.RunTime,
		}
		if test.Message != "" || test.StackTrace != "" {
			run.Exception = &OracleException{
				Type:    exceptionType(test.Message),
				Message: test.Message,
				Stack:   test.StackTrace,
			}
			run.Stack = stackFramesFromString(test.StackTrace)
		}
		if test.QueueItemID != "" {
			run.RawArtifacts = append(run.RawArtifacts, OracleArtifact{Type: "ApexTestQueueItem", Raw: test.QueueItemID})
		}
		runs = append(runs, NormalizeRun(run))
	}
	return runs, nil
}

func OracleRunFromToolingSnippet(project, orgAlias string, result probe.ToolingSnippetResult) OracleRun {
	status := OracleStatusPass
	if !result.Compiled {
		status = OracleStatusCompileError
	} else if !result.Success {
		status = OracleStatusRuntimeError
	}
	obs := ParseApexLog(result.RawLogs)
	run := OracleRun{
		SchemaVersion: SchemaVersion,
		Source:        "salesforce",
		Project:       project,
		OrgAlias:      orgAlias,
		TestClass:     result.ID,
		Status:        status,
		Events:        obs.Events,
		DebugPayloads: obs.DebugPayloads,
		Limits:        obs.Limits,
	}
	if result.ExceptionType != "" || result.ExceptionMessage != "" {
		run.Exception = &OracleException{Type: result.ExceptionType, Message: result.ExceptionMessage, Stack: result.ExceptionStackTrace}
		run.Stack = stackFramesFromString(result.ExceptionStackTrace)
	}
	if result.RawLogs != "" {
		run.RawArtifacts = append(run.RawArtifacts, OracleArtifact{Type: "ApexLog", Raw: result.RawLogs})
	}
	return NormalizeRun(run)
}

func ParseApexLog(log string) ApexLogObservation {
	log = strings.ReplaceAll(log, `\n`, "\n")
	lines := strings.Split(log, "\n")
	obs := ApexLogObservation{}
	sequence := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		kind := parts[1]
		nextSequence := func() int {
			sequence++
			return sequence
		}
		switch kind {
		case "METHOD_ENTRY", "METHOD_EXIT":
			sequence := nextSequence()
			obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventMethodCall, Sequence: sequence, Name: lastPart(parts), Raw: line}))
		case "SOQL_EXECUTE_BEGIN":
			sequence := nextSequence()
			obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventSOQL, Sequence: sequence, Query: lastPart(parts), Raw: line}))
		case "DML_BEGIN", "DML_END":
			sequence := nextSequence()
			obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventDML, Sequence: sequence, Operation: logField(parts, "Op"), Object: logField(parts, "Type"), Raw: line}))
		case "USER_DEBUG":
			sequence := nextSequence()
			event := NormalizeEvent(OracleEvent{Type: OracleEventDebug, Sequence: sequence, Name: "USER_DEBUG", Message: lastPart(parts), Raw: line})
			if payload, ok := parseOracleDebugPayload(sequence, lastPart(parts)); ok {
				obs.DebugPayloads = append(obs.DebugPayloads, payload)
				if value, ok := payload.Value.(map[string]any); ok {
					event.Payload = normalizeMap(value)
				}
			}
			obs.Events = append(obs.Events, event)
		case "EXCEPTION_THROWN", "FATAL_ERROR":
			sequence := nextSequence()
			message := lastPart(parts)
			obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventException, Sequence: sequence, ExceptionType: exceptionType(message), Message: message, Raw: line}))
		case "LIMIT_USAGE_FOR_NS", "LIMIT_USAGE":
			sequence := nextSequence()
			limit := OracleLimit{Name: normalizeString(lastPart(parts)), Sequence: sequence}
			obs.Limits = append(obs.Limits, limit)
			obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventLimit, Sequence: sequence, Name: limit.Name, Raw: line}))
		default:
			if strings.HasPrefix(kind, "WF_") {
				sequence := nextSequence()
				obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventWorkflow, Sequence: sequence, Name: kind, Raw: line}))
			} else if strings.HasPrefix(kind, "FLOW_") {
				sequence := nextSequence()
				obs.Events = append(obs.Events, NormalizeEvent(OracleEvent{Type: OracleEventFlow, Sequence: sequence, Name: kind, Raw: line}))
			}
		}
	}
	return obs
}

func parseOracleDebugPayload(sequence int, message string) (OracleDebugPayload, bool) {
	const marker = "OAER_ORACLE:"
	idx := strings.Index(message, marker)
	if idx < 0 {
		return OracleDebugPayload{}, false
	}
	raw := strings.TrimSpace(message[idx+len(marker):])
	var payload struct {
		Label string `json:"label"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil && (payload.Label != "" || payload.Value != nil) {
		return OracleDebugPayload{Label: normalizeString(payload.Label), Sequence: sequence, Value: NormalizeValue(payload.Value), Raw: normalizeString(raw)}, true
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return OracleDebugPayload{Sequence: sequence, Value: normalizeString(raw), Raw: normalizeString(raw)}, true
	}
	return OracleDebugPayload{Sequence: sequence, Value: NormalizeValue(value), Raw: normalizeString(raw)}, true
}

func statusFromSalesforceOutcome(outcome string) OracleStatus {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "pass", "passed":
		return OracleStatusPass
	case "fail", "failed":
		return OracleStatusFail
	case "skip", "skipped":
		return OracleStatusSkipped
	case "compilefail", "compile_failure", "compile_error":
		return OracleStatusCompileError
	default:
		if strings.TrimSpace(outcome) == "" {
			return OracleStatusInfrastructureError
		}
		return OracleStatusRuntimeError
	}
}

func stackFramesFromString(stack string) []OracleStackFrame {
	if strings.TrimSpace(stack) == "" {
		return nil
	}
	lines := strings.Split(stack, "\n")
	frames := make([]OracleStackFrame, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		frames = append(frames, OracleStackFrame{Symbol: normalizeString(line)})
	}
	return frames
}

func exceptionType(message string) string {
	message = strings.TrimSpace(message)
	if idx := strings.Index(message, ":"); idx > 0 {
		return strings.TrimSpace(message[:idx])
	}
	fields := strings.Fields(message)
	if len(fields) > 0 && strings.Contains(fields[0], "Exception") {
		return fields[0]
	}
	return ""
}

func logField(parts []string, name string) string {
	prefix := name + ":"
	for _, part := range parts {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(part, prefix))
		}
	}
	return ""
}

func lastPart(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func DefaultRunID(now time.Time) string {
	return now.UTC().Format("20060102T150405Z")
}
