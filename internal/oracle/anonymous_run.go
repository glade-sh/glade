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

// DefaultAnonChunkSize bounds how many probes share a single anonymous Apex
// execution. Anonymous Apex runs under governor limits, so probes are chunked
// rather than concatenated without bound.
const DefaultAnonChunkSize = 50

// BuildAnonymousProbeScript inlines a set of probes into one anonymous Apex
// script. Each probe runs in its own block so a failure in one is caught and
// reported without aborting the rest, and every block emits a single
// GLADE_ORACLE: debug line carrying its probeId. This is the synchronous
// counterpart to the per-class @IsTest probe: one `sf apex run` executes the
// whole chunk in seconds instead of queuing async test jobs.
func BuildAnonymousProbeScript(items []WorkItem) string {
	var b strings.Builder
	for _, item := range items {
		probeID := apexQuote(item.ProbeID)
		surfaceID := apexQuote(item.SurfaceID)
		area := apexQuote(item.Area)
		b.WriteString("{\n")
		b.WriteString("    try {\n")
		b.WriteString("        Map<String, Object> p = new Map<String, Object>();\n")
		fmt.Fprintf(&b, "        p.put('probeId', %s);\n", probeID)
		fmt.Fprintf(&b, "        p.put('surfaceId', %s);\n", surfaceID)
		fmt.Fprintf(&b, "        p.put('area', %s);\n", area)
		b.WriteString("        p.put('status', 'generated');\n")
		b.WriteString("        p.put('result', null);\n")
		b.WriteString("        p.put('exceptionType', null);\n")
		b.WriteString("        p.put('exceptionMessage', null);\n")
		fmt.Fprintf(&b, "        System.debug(LoggingLevel.ERROR, '%s' + JSON.serialize(p));\n", anonProbeMarker)
		b.WriteString("    } catch (Exception probeEx) {\n")
		b.WriteString("        Map<String, Object> p = new Map<String, Object>();\n")
		fmt.Fprintf(&b, "        p.put('probeId', %s);\n", probeID)
		b.WriteString("        p.put('status', 'exception');\n")
		b.WriteString("        p.put('exceptionType', probeEx.getTypeName());\n")
		b.WriteString("        p.put('exceptionMessage', probeEx.getMessage());\n")
		fmt.Fprintf(&b, "        System.debug(LoggingLevel.ERROR, '%s' + JSON.serialize(p));\n", anonProbeMarker)
		b.WriteString("    }\n")
		b.WriteString("}\n")
	}
	return b.String()
}

// apexQuote renders a Go string as a single-quoted Apex string literal.
func apexQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return "'" + s + "'"
}

type anonProbePayload struct {
	ProbeID          string `json:"probeId"`
	SurfaceID        string `json:"surfaceId"`
	Area             string `json:"area"`
	Status           string `json:"status"`
	ExceptionType    string `json:"exceptionType"`
	ExceptionMessage string `json:"exceptionMessage"`
}

// extractApexRunLog pulls the debug log text out of `sf apex run --json` stdout.
func extractApexRunLog(stdout []byte) (string, error) {
	var parsed struct {
		Result struct {
			Logs     string `json:"logs"`
			Success  bool   `json:"success"`
			Compiled bool   `json:"compiled"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout, &parsed); err != nil {
		return "", fmt.Errorf("parse sf apex run json: %w", err)
	}
	return parsed.Result.Logs, nil
}

// AnonymousProbeRunsFromLog maps the GLADE_ORACLE: debug lines in a synchronous
// run log back to per-probe observations. Each probe is keyed by probeId to its
// WorkItem so the resulting OracleRun carries the same TestClass/TestMethod the
// local glade run uses, keeping the diff alignment unchanged. Probes that never
// reported (a compile gap or aborted block) are emitted as infrastructure
// errors so they are visible rather than silently dropped.
func AnonymousProbeRunsFromLog(project, orgAlias string, items []WorkItem, logText string) []OracleRun {
	byProbe := make(map[string]WorkItem, len(items))
	for _, item := range items {
		byProbe[item.ProbeID] = item
	}

	seen := make(map[string]bool, len(items))
	runs := make([]OracleRun, 0, len(items))

	for _, line := range strings.Split(logText, "\n") {
		idx := strings.Index(line, anonProbeMarker)
		if idx < 0 {
			continue
		}
		raw := strings.TrimSpace(line[idx+len(anonProbeMarker):])
		var payload anonProbePayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		item, ok := byProbe[payload.ProbeID]
		if !ok {
			continue
		}
		seen[payload.ProbeID] = true
		runs = append(runs, anonRunFromPayload(project, orgAlias, item, payload))
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
			Exception:     &OracleException{Message: "probe produced no GLADE_ORACLE observation"},
		})
	}
	return runs
}

func anonRunFromPayload(project, orgAlias string, item WorkItem, payload anonProbePayload) OracleRun {
	run := OracleRun{
		SchemaVersion: SchemaVersion,
		Source:        "salesforce",
		Project:       project,
		OrgAlias:      orgAlias,
		TestClass:     item.GeneratedClass,
		TestMethod:    item.MethodName,
		Status:        OracleStatusPass,
	}
	if strings.EqualFold(payload.Status, "exception") {
		run.Status = OracleStatusRuntimeError
		run.Exception = &OracleException{
			Type:    payload.ExceptionType,
			Message: payload.ExceptionMessage,
		}
	}
	return run
}

// RunAnonymousProbes executes a shard's probes synchronously via anonymous Apex
// in bounded chunks and returns one observation per probe. It performs no
// metadata deploy and never falls back to per-class async test runs.
func (r SalesforceRunner) RunAnonymousProbes(ctx context.Context, project, orgAlias string, items []WorkItem, chunkSize int) ([]OracleRun, error) {
	if strings.TrimSpace(orgAlias) == "" {
		return nil, fmt.Errorf("target org is required")
	}
	if chunkSize <= 0 {
		chunkSize = DefaultAnonChunkSize
	}
	runner := r.CommandRunner
	if runner == nil {
		runner = ExecCommandRunner{}
	}

	all := make([]OracleRun, 0, len(items))
	for start := 0; start < len(items); start += chunkSize {
		end := start + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]
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
		if runErr != nil && len(stdout) == 0 {
			return nil, fmt.Errorf("sf apex run failed: %w (stderr: %s)", runErr, strings.TrimSpace(string(stderr)))
		}
		logText, err := extractApexRunLog(stdout)
		if err != nil {
			return nil, err
		}
		all = append(all, AnonymousProbeRunsFromLog(project, orgAlias, chunk, logText)...)
	}
	return all, nil
}
