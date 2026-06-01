package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// anonProbeMarker is the debug line prefix each inlined probe emits. It matches
// the marker the log parser already recognizes elsewhere in this package.
const anonProbeMarker = "GLADE_ORACLE:"

// anonScriptByteBudget bounds the size of a single anonymous Apex execution.
// Salesforce rejects an oversized anonymous block with "Script too large", and
// because anonymous Apex compiles atomically, one oversized chunk loses every
// probe in it. The budget is kept well under the platform limit so a chunk
// always compiles.
const anonScriptByteBudget = 15000

// DefaultAnonChunkSize is a fallback probe-count cap used when a caller does not
// supply one. Chunking is primarily driven by anonScriptByteBudget; this only
// guards against pathologically long single lines.
const DefaultAnonChunkSize = 100

// probePayload is the per-probe observation emitted into the debug log. The
// probe body resolves the surface type at runtime via Type.forName and records
// the real status: "resolved" when the org knows the type, "missing" when it
// does not, or "exception" when resolution threw. This is genuine org signal,
// not a constant.
type probePayload struct {
	ProbeID          string `json:"probeId"`
	SurfaceID        string `json:"surfaceId,omitempty"`
	Area             string `json:"area,omitempty"`
	Status           string `json:"status"`
	ExceptionType    string `json:"exceptionType,omitempty"`
	ExceptionMessage string `json:"exceptionMessage,omitempty"`
}

// BuildAnonymousProbeScript inlines a set of probes into one anonymous Apex
// script, one self-contained block per probe. This is the synchronous
// counterpart to the per-class @IsTest probe: one `sf apex run` executes the
// whole chunk in seconds. Each block resolves its surface type reflectively, so
// it compiles for any value and one probe never fails the rest of the chunk.
// Callers must keep a chunk within anonScriptByteBudget; AnonymousChunks does
// that splitting.
func BuildAnonymousProbeScript(items []WorkItem) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString(anonProbeBlock(probeTargetFromWorkItem(item)))
	}
	return b.String()
}

// AnonymousChunks splits items into groups whose rendered script stays within
// the byte budget (and an optional probe-count cap), so each group compiles as
// one anonymous block.
func AnonymousChunks(items []WorkItem, maxCount int) [][]WorkItem {
	if maxCount <= 0 {
		maxCount = DefaultAnonChunkSize
	}
	var chunks [][]WorkItem
	var current []WorkItem
	currentBytes := 0
	for _, item := range items {
		lineBytes := len(anonProbeBlock(probeTargetFromWorkItem(item)))
		if len(current) > 0 && (currentBytes+lineBytes > anonScriptByteBudget || len(current) >= maxCount) {
			chunks = append(chunks, current)
			current = nil
			currentBytes = 0
		}
		current = append(current, item)
		currentBytes += lineBytes
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

// apexRunResult is the normalized outcome of one `sf apex run --json`, tolerant
// of both the success shape (top-level "result") and the failure shape
// (top-level "data"), which the CLI uses when compilation fails.
type apexRunResult struct {
	Success        bool
	Compiled       bool
	Logs           string
	CompileProblem string
}

type apexRunPayload struct {
	Success        bool   `json:"success"`
	Compiled       bool   `json:"compiled"`
	Logs           string `json:"logs"`
	CompileProblem string `json:"compileProblem"`
}

func parseApexRunResult(stdout []byte) (apexRunResult, error) {
	var envelope struct {
		Result *apexRunPayload `json:"result"`
		Data   *apexRunPayload `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		return apexRunResult{}, fmt.Errorf("parse sf apex run json: %w", err)
	}
	payload := envelope.Result
	if payload == nil {
		payload = envelope.Data
	}
	if payload == nil {
		return apexRunResult{}, fmt.Errorf("sf apex run json missing result and data")
	}
	return apexRunResult{
		Success:        payload.Success,
		Compiled:       payload.Compiled,
		Logs:           payload.Logs,
		CompileProblem: payload.CompileProblem,
	}, nil
}

// AnonymousProbeRunsFromResult maps the GLADE_ORACLE: debug lines in a
// synchronous run back to per-probe observations. Each probe is keyed by probeId
// to its WorkItem so the resulting OracleRun carries the same
// TestClass/TestMethod the local glade run uses, keeping diff alignment intact.
// Probes that never reported are surfaced as infrastructure errors (carrying the
// chunk's compile problem when the whole block failed to compile) rather than
// silently dropped.
func AnonymousProbeRunsFromResult(project, orgAlias string, items []WorkItem, result apexRunResult) []OracleRun {
	byProbe := make(map[string]WorkItem, len(items))
	for _, item := range items {
		byProbe[item.ProbeID] = item
	}

	seen := make(map[string]bool, len(items))
	runs := make([]OracleRun, 0, len(items))

	for _, line := range strings.Split(result.Logs, "\n") {
		idx := strings.Index(line, anonProbeMarker)
		if idx < 0 {
			continue
		}
		raw := strings.TrimSpace(line[idx+len(anonProbeMarker):])
		var payload probePayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		item, ok := byProbe[payload.ProbeID]
		if !ok || seen[payload.ProbeID] {
			continue
		}
		seen[payload.ProbeID] = true
		runs = append(runs, anonRunFromPayload(project, orgAlias, item, payload))
	}

	missingMessage := "probe produced no GLADE_ORACLE observation"
	if strings.TrimSpace(result.CompileProblem) != "" {
		missingMessage = "anonymous chunk failed to compile: " + truncate(result.CompileProblem, 300)
	}
	for _, item := range items {
		if seen[item.ProbeID] {
			continue
		}
		runs = append(runs, OracleRun{
			SchemaVersion: SchemaVersion,
			Source:        "salesforce",
			Project:       project,
			OrgAlias:      orgAlias,
			TestClass:     item.GeneratedClass,
			TestMethod:    item.MethodName,
			Status:        OracleStatusInfrastructureError,
			Exception:     &OracleException{Message: missingMessage},
		})
	}
	return runs
}

func anonRunFromPayload(project, orgAlias string, item WorkItem, payload probePayload) OracleRun {
	run := OracleRun{
		SchemaVersion: SchemaVersion,
		Source:        "salesforce",
		Project:       project,
		OrgAlias:      orgAlias,
		TestClass:     item.GeneratedClass,
		TestMethod:    item.MethodName,
		Status:        OracleStatusPass,
	}
	switch strings.ToLower(payload.Status) {
	case "exception":
		run.Status = OracleStatusRuntimeError
		run.Exception = &OracleException{
			Type:    payload.ExceptionType,
			Message: payload.ExceptionMessage,
		}
	case "missing":
		// The type did not resolve on the org. Use the shared missing-type
		// signal so this lines up with a glade probe whose resolution
		// assertion also failed. Both-missing then matches; a divergence only
		// fires when one side resolves the type and the other does not.
		run.Status = OracleStatusFail
		run.Exception = missingTypeException()
	}
	return run
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// RunAnonymousProbes executes a shard's probes synchronously via anonymous Apex
// in size-bounded chunks and returns one observation per probe. It performs no
// metadata deploy and never falls back to per-class async test runs.
func (r SalesforceRunner) RunAnonymousProbes(ctx context.Context, project, orgAlias string, items []WorkItem, chunkSize int) ([]OracleRun, error) {
	if strings.TrimSpace(orgAlias) == "" {
		return nil, fmt.Errorf("target org is required")
	}
	runner := r.CommandRunner
	if runner == nil {
		runner = ExecCommandRunner{}
	}

	all := make([]OracleRun, 0, len(items))
	for _, chunk := range AnonymousChunks(items, chunkSize) {
		script := BuildAnonymousProbeScript(chunk)

		tmp, err := os.MkdirTemp("", "glade-anon-probe-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		scriptPath := filepath.Join(tmp, "probe.apex")
		if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("write anon script: %w", err)
		}
		stdout, stderr, runErr := runner.Run(ctx, project, "sf",
			"apex", "run",
			"--target-org", orgAlias,
			"--file", scriptPath,
			"--json",
		)
		os.RemoveAll(tmp)
		if len(stdout) == 0 {
			return nil, fmt.Errorf("sf apex run produced no output: %w (stderr: %s)", runErr, strings.TrimSpace(string(stderr)))
		}
		result, err := parseApexRunResult(stdout)
		if err != nil {
			return nil, err
		}
		all = append(all, AnonymousProbeRunsFromResult(project, orgAlias, chunk, result)...)
	}
	return all, nil
}
