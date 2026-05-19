package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/open-aer/oaer/internal/capability"
)

// SFDXExecutor captures golden responses by running probes against a real
// Salesforce org via the sfdx/sf CLI.
type SFDXExecutor struct {
	OrgAlias        string
	OrgShape        map[string]interface{}
	CaptureDebugLog bool
	DebugLogs       []ProbeDebugLog
}

const maxGoldenBatchSize = 20
const maxIsolatedGoldenBatchSize = 5
const apexRunTimeout = 90 * time.Second

// PreflightShape captures the scratch org shape before golden execution.
func (s *SFDXExecutor) PreflightShape(probeDir string) (map[string]interface{}, error) {
	jsonStr, _, err := s.runProbeCodeJSON(probeDir, "System.assert(false, 'OAER_PROBE:' + ProbeRunner.preflight());", probeRunTimeout())
	if err != nil {
		return nil, err
	}
	var shape map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &shape); err != nil {
		return nil, fmt.Errorf("parse preflight shape %q: %w", jsonStr, err)
	}
	return shape, nil
}

// CaptureGolden runs each probe ID against the scratch org and returns the
// parsed responses.
func (s *SFDXExecutor) CaptureGolden(probeDir string, probeIDs []string) (map[string]ProbeResult, []ProbeTiming, error) {
	results := make(map[string]ProbeResult, len(probeIDs))
	timings := make([]ProbeTiming, 0, len(probeIDs))
	maxBatch, maxIsolatedBatch := probeBatchSizes()
	timeout := probeRunTimeout()
	verbose := probeVerbose()
	deployer := NewDeployer(probeDir, s.OrgAlias)
	if err := deployer.ResetProbeDataWithTimeout(timeout, io.Discard); err != nil {
		return nil, nil, fmt.Errorf("reset probe data before golden capture: %w", err)
	}
	shape, err := s.PreflightShape(probeDir)
	if err != nil {
		return nil, nil, fmt.Errorf("org shape preflight: %w", err)
	}
	s.OrgShape = shape
	for i := 0; i < len(probeIDs); {
		id := probeIDs[i]
		if shouldSkipProbe(id) {
			fmt.Printf("  golden %d-%d/%d skipped\n", i+1, i+1, len(probeIDs))
			results[id] = ProbeResult{
				ProbeID:          id,
				Category:         "Skipped",
				Result:           nil,
				ExceptionType:    strPtr("SkippedProbe"),
				ExceptionMessage: strPtr("probe skipped by OAER_PROBE_SKIP_IDS"),
			}
			timings = append(timings, ProbeTiming{Phase: "golden", ProbeID: id, Mode: "skipped", DurationMS: 0})
			i++
			continue
		}
		if isStubContractProbeID(id) {
			fmt.Printf("  golden %d-%d/%d single %s\n", i+1, i+1, len(probeIDs), id)
			start := time.Now()
			result, err := s.runProbe(probeDir, id)
			if err != nil {
				errType := "ApexExecutionError"
				errMsg := err.Error()
				result = ProbeResult{
					ProbeID:          id,
					Category:         "Stub Contracts",
					Result:           nil,
					ExceptionType:    &errType,
					ExceptionMessage: &errMsg,
				}
			}
			results[id] = result
			timings = append(timings, ProbeTiming{Phase: "golden", ProbeID: id, Mode: "single", DurationMS: time.Since(start).Milliseconds()})
			i++
			continue
		}
		spec := probeSpecByID(id)
		if spec.CanBatch {
			batch := []string{id}
			for i+len(batch) < len(probeIDs) && len(batch) < maxBatch {
				next := probeIDs[i+len(batch)]
				if !probeSpecByID(next).CanBatch {
					break
				}
				batch = append(batch, next)
			}
			fmt.Printf("  golden %d-%d/%d batch\n", i+1, i+len(batch), len(probeIDs))
			if verbose {
				fmt.Printf("    ids: %s\n", strings.Join(batch, ", "))
			}
			start := time.Now()
			if len(batch) == 1 {
				result, err := s.runProbe(probeDir, batch[0])
				if err != nil {
					errType := "ApexExecutionError"
					errMsg := err.Error()
					result = ProbeResult{
						ProbeID:          batch[0],
						Category:         "Stdlib & System",
						Result:           nil,
						ExceptionType:    &errType,
						ExceptionMessage: &errMsg,
					}
				}
				results[result.ProbeID] = result
			} else {
				batchResults, logText, err := s.runProbeBatch(probeDir, batch, timeout)
				if err != nil {
					return nil, nil, fmt.Errorf("probe batch %v: %w", batch, err)
				}
				for _, result := range batchResults {
					results[result.ProbeID] = result
				}
				s.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "batch", Log: logText})
			}
			timings = append(timings, ProbeTiming{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "batch", DurationMS: time.Since(start).Milliseconds()})
			i += len(batch)
			continue
		}
		batch := []string{id}
		hasStateful := spec.Isolation == ProbeIsolationStateful
		for i+len(batch) < len(probeIDs) && len(batch) < maxIsolatedBatch {
			next := probeIDs[i+len(batch)]
			nextSpec := probeSpecByID(next)
			if nextSpec.CanBatch {
				break
			}
			if nextSpec.Isolation == ProbeIsolationStateful {
				hasStateful = true
			}
			batch = append(batch, next)
		}
		fmt.Printf("  golden %d-%d/%d isolated\n", i+1, i+len(batch), len(probeIDs))
		if verbose {
			fmt.Printf("    ids: %s\n", strings.Join(batch, ", "))
		}
		if hasStateful {
			if err := deployer.ResetProbeDataWithTimeout(timeout, io.Discard); err != nil {
				return nil, nil, fmt.Errorf("reset probe data before isolated batch %v: %w", batch, err)
			}
		}
		start := time.Now()
		if len(batch) == 1 {
			result, err := s.runProbe(probeDir, batch[0])
			if err != nil {
				errType := "ApexExecutionError"
				errMsg := err.Error()
				result = ProbeResult{
					ProbeID:          batch[0],
					Category:         "Stdlib & System",
					Result:           nil,
					ExceptionType:    &errType,
					ExceptionMessage: &errMsg,
				}
			}
			results[result.ProbeID] = result
		} else {
			batchResults, logText, err := s.runProbeBatchIsolated(probeDir, batch, timeout)
			if err != nil {
				return nil, nil, fmt.Errorf("probe isolated batch %v: %w", batch, err)
			}
			for _, result := range batchResults {
				results[result.ProbeID] = result
			}
			s.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "isolated_batch", Log: logText})
		}
		timings = append(timings, ProbeTiming{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "isolated_batch", DurationMS: time.Since(start).Milliseconds()})
		i += len(batch)
	}
	return results, timings, nil
}

func probeBatchSizes() (int, int) {
	maxBatch := maxGoldenBatchSize
	maxIsolatedBatch := maxIsolatedGoldenBatchSize
	if v := strings.TrimSpace(os.Getenv("OAER_PROBE_MAX_BATCH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxBatch = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("OAER_PROBE_MAX_ISOLATED_BATCH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxIsolatedBatch = n
		}
	}
	return maxBatch, maxIsolatedBatch
}

func (s *SFDXExecutor) runProbe(probeDir, probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.assert(false, 'OAER_PROBE:' + ProbeRunner.run('%s'));", probeID)
	var stubSpec capability.StubContractProbeSpec
	isStub := false
	if isStubContractProbeID(probeID) {
		spec, ok := stubContractProbeSpecByID(probeID)
		if !ok {
			return ProbeResult{}, fmt.Errorf("missing generated stub contract probe spec for %s", probeID)
		}
		stubSpec = spec
		isStub = true
		code = buildOrgStubContractProbeCode(spec)
	}
	jsonStr, logs, err := s.runProbeCodeJSON(probeDir, code, probeRunTimeout())
	if err != nil {
		if isStub {
			if result, ok := stubContractCompileFailureResult(stubSpec, err); ok {
				return result, nil
			}
		}
		if strings.Contains(logs, "daily usage limit of apex log headers") {
			return ProbeResult{}, fmt.Errorf("assertion probe unexpectedly requested debug logs: %w", err)
		}
		return ProbeResult{}, err
	}
	var result ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ProbeResult{}, fmt.Errorf("parse probe result %q: %w", jsonStr, err)
	}
	if s.CaptureDebugLog && logs != "" {
		s.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeID: probeID, Mode: "single", Log: logs})
	}
	return result, nil
}

func (s *SFDXExecutor) runProbeBatch(probeDir string, probeIDs []string, timeout time.Duration) ([]ProbeResult, string, error) {
	code := fmt.Sprintf("System.assert(false, 'OAER_PROBE:' + ProbeRunner.runMany(new List<String>{%s}));", apexStringList(probeIDs))
	jsonStr, logs, err := s.runProbeCodeJSON(probeDir, code, timeout)
	if err != nil {
		if strings.Contains(logs, "daily usage limit of apex log headers") {
			return nil, logs, fmt.Errorf("assertion batch unexpectedly requested debug logs: %w", err)
		}
		if err != nil {
			return nil, logs, err
		}
	}
	var results []ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, logs, fmt.Errorf("parse probe batch result %q: %w", jsonStr, err)
	}
	return results, logs, nil
}

func (s *SFDXExecutor) runProbeBatchIsolated(probeDir string, probeIDs []string, timeout time.Duration) ([]ProbeResult, string, error) {
	code := fmt.Sprintf("System.assert(false, 'OAER_PROBE:' + ProbeRunner.runManyIsolated(new List<String>{%s}));", apexStringList(probeIDs))
	jsonStr, logs, err := s.runProbeCodeJSON(probeDir, code, timeout)
	if err != nil {
		if strings.Contains(logs, "daily usage limit of apex log headers") {
			return nil, logs, fmt.Errorf("assertion isolated batch unexpectedly requested debug logs: %w", err)
		}
		if err != nil {
			return nil, logs, err
		}
	}
	var results []ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, logs, fmt.Errorf("parse isolated probe batch result %q: %w", jsonStr, err)
	}
	return results, logs, nil
}

func (s *SFDXExecutor) appendDebugLog(entry ProbeDebugLog) {
	if !s.CaptureDebugLog || strings.TrimSpace(entry.Log) == "" {
		return
	}
	s.DebugLogs = append(s.DebugLogs, entry)
}

func apexStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+strings.ReplaceAll(value, "'", "\\'")+"'")
	}
	return strings.Join(quoted, ",")
}

func (s *SFDXExecutor) runProbeCodeJSON(probeDir, code string, timeout time.Duration) (string, string, error) {
	// Try modern sf CLI first (supports --code inline), then fall back to legacy sfdx.
	outputBytes, usedCmd, err := s.runWithSF(probeDir, code, timeout)
	if err != nil {
		if strings.Contains(err.Error(), "timed out") {
			return "", "", err
		}
		outputBytes, usedCmd, err = s.runWithSFDX(probeDir, code, timeout)
		if err != nil {
			return "", "", err
		}
	}

	// sf CLI returns different JSON shapes on success vs error.
	// Success: {"result": {"success": true, "logs": "..."}}
	// Error:   {"data": {"success": false, "logs": "...", "exceptionMessage": "..."}}
	type sfdxResult struct {
		Success          bool   `json:"success"`
		Compiled         bool   `json:"compiled"`
		Executed         bool   `json:"executed"`
		Logs             string `json:"logs"`
		CompileProblem   string `json:"compileProblem"`
		ExceptionMessage string `json:"exceptionMessage"`
	}
	type sfdxOutput struct {
		Status int        `json:"status"`
		Result sfdxResult `json:"result"`
		Data   sfdxResult `json:"data"`
	}

	var output sfdxOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return "", "", fmt.Errorf("parse %s output: %w (raw: %s)", usedCmd, err, string(outputBytes))
	}

	r := output.Result
	if r.Logs == "" && !r.Success {
		r = output.Data
	}

	if !r.Success {
		if !r.Compiled && strings.TrimSpace(r.CompileProblem) != "" {
			return "", r.Logs, &apexCompileError{Problem: r.CompileProblem}
		}
		if jsonStr, ok := extractAssertionJSON(r.ExceptionMessage); ok {
			return jsonStr, r.Logs, nil
		}
		return "", r.Logs, fmt.Errorf("apex execution failed via %s: compiled=%v executed=%v logs=%q exc=%q", usedCmd, r.Compiled, r.Executed, r.Logs, r.ExceptionMessage)
	}

	jsonStr, err := extractDebugJSON(r.Logs)
	if err != nil {
		return "", r.Logs, fmt.Errorf("extract debug JSON from %s: %w (raw logs: %q)", usedCmd, err, output.Result.Logs)
	}

	return jsonStr, r.Logs, nil
}

// runWithSF uses the modern "sf" CLI which requires a file path (-f).
func (s *SFDXExecutor) runWithSF(probeDir, code string, timeout time.Duration) ([]byte, string, error) {
	tmpDir, err := os.MkdirTemp("", "oaer-probe-sf-*")
	if err != nil {
		return nil, "sf", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "probe.apex")
	if err := os.WriteFile(tmpFile, []byte(code), 0o644); err != nil {
		return nil, "sf", fmt.Errorf("write temp file: %w", err)
	}

	cmd := exec.Command("sf", "apex", "run", "--target-org", s.OrgAlias, "--file", tmpFile, "--json")
	cmd.Dir = probeDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runWithHardTimeout(cmd, &stdout, &stderr, timeout); err != nil {
		if stdout.Len() == 0 {
			return nil, "sf", fmt.Errorf("sf execution failed: %w (stderr: %s)", err, stderr.String())
		}
	}
	return stdout.Bytes(), "sf", nil
}

// runWithSFDX uses the legacy "sfdx" CLI which requires a file path (-f).
func (s *SFDXExecutor) runWithSFDX(probeDir, code string, timeout time.Duration) ([]byte, string, error) {
	tmpDir, err := os.MkdirTemp("", "oaer-probe-*")
	if err != nil {
		return nil, "sfdx", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "probe.apex")
	if err := os.WriteFile(tmpFile, []byte(code), 0o644); err != nil {
		return nil, "sfdx", fmt.Errorf("write temp file: %w", err)
	}

	cmd := exec.Command("sfdx", "force:apex:execute", "--targetusername", s.OrgAlias, "--apexcodefile", tmpFile, "--json")
	cmd.Dir = probeDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runWithHardTimeout(cmd, &stdout, &stderr, timeout); err != nil {
		if stdout.Len() == 0 {
			return nil, "sfdx", fmt.Errorf("sfdx execution failed: %w (stderr: %s)", err, stderr.String())
		}
	}
	return stdout.Bytes(), "sfdx", nil
}

func runWithHardTimeout(cmd *exec.Cmd, stdout, stderr *bytes.Buffer, timeout time.Duration) error {
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		return fmt.Errorf("execution timed out after %s", timeout)
	}
}

func probeRunTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("OAER_APEX_RUN_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return apexRunTimeout
}

func probeVerbose() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("OAER_PROBE_VERBOSE")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func shouldSkipProbe(id string) bool {
	raw := strings.TrimSpace(os.Getenv("OAER_PROBE_SKIP_IDS"))
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == id {
			return true
		}
	}
	return false
}

var debugLogPattern = regexp.MustCompile(`USER_DEBUG\|\[\d+\]\|DEBUG\|(.+)`)

func extractDebugJSON(logs string) (string, error) {
	// Salesforce sometimes returns logs with escaped newlines; normalize them.
	logs = strings.ReplaceAll(logs, `\n`, "\n")
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		matches := debugLogPattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("no USER_DEBUG line found in logs")
}

func extractAssertionJSON(message string) (string, bool) {
	const marker = "OAER_PROBE:"
	idx := strings.Index(message, marker)
	if idx < 0 {
		return "", false
	}
	return message[idx+len(marker):], true
}
