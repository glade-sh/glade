package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/capability"
)

// RestExecutor captures golden responses using Tooling API executeAnonymous.
type RestExecutor struct {
	OrgAlias        string
	OrgShape        map[string]interface{}
	CaptureDebugLog bool
	DebugLogs       []ProbeDebugLog

	instanceURL string
	accessToken string
	client      *http.Client
}

func (r *RestExecutor) CaptureGolden(probeDir string, probeIDs []string) (map[string]ProbeResult, []ProbeTiming, error) {
	if err := r.ensureAuth(); err != nil {
		return nil, nil, err
	}
	results := make(map[string]ProbeResult, len(probeIDs))
	timings := make([]ProbeTiming, 0, len(probeIDs))
	maxBatch, maxIsolatedBatch := probeBatchSizes()
	timeout := probeRunTimeout()
	verbose := probeVerbose()
	deployer := NewDeployer(probeDir, r.OrgAlias)
	if err := deployer.ResetProbeDataWithTimeout(timeout, io.Discard); err != nil {
		return nil, nil, fmt.Errorf("reset probe data before golden capture: %w", err)
	}
	shape, err := r.preflightShape()
	if err != nil {
		return nil, nil, fmt.Errorf("org shape preflight: %w", err)
	}
	r.OrgShape = shape
	for i := 0; i < len(probeIDs); {
		id := probeIDs[i]
		if shouldSkipProbe(id) {
			fmt.Printf("  golden %d-%d/%d skipped\n", i+1, i+1, len(probeIDs))
			results[id] = ProbeResult{
				ProbeID:          id,
				Category:         "Skipped",
				Result:           nil,
				ExceptionType:    strPtr("SkippedProbe"),
				ExceptionMessage: strPtr("probe skipped by GLADE_PROBE_SKIP_IDS"),
			}
			timings = append(timings, ProbeTiming{Phase: "golden", ProbeID: id, Mode: "skipped", DurationMS: 0})
			i++
			continue
		}
		if isStubContractProbeID(id) {
			fmt.Printf("  golden %d-%d/%d single %s\n", i+1, i+1, len(probeIDs), id)
			start := time.Now()
			result, err := r.runProbe(id)
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
				result, err := r.runProbe(batch[0])
				if err != nil {
					errType := "ApexExecutionError"
					errMsg := err.Error()
					result = ProbeResult{
						ProbeID:          batch[0],
						Category:         probeSpecByID(batch[0]).Category,
						Result:           nil,
						ExceptionType:    &errType,
						ExceptionMessage: &errMsg,
					}
				}
				results[result.ProbeID] = result
			} else {
				batchResults, logText, err := r.runProbeBatch(batch)
				if err != nil {
					return nil, nil, fmt.Errorf("probe batch %v: %w", batch, err)
				}
				for _, result := range batchResults {
					results[result.ProbeID] = result
				}
				r.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "batch", Log: logText})
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
			result, err := r.runProbe(batch[0])
			if err != nil {
				errType := "ApexExecutionError"
				errMsg := err.Error()
				result = ProbeResult{
					ProbeID:          batch[0],
					Category:         probeSpecByID(batch[0]).Category,
					Result:           nil,
					ExceptionType:    &errType,
					ExceptionMessage: &errMsg,
				}
			}
			results[result.ProbeID] = result
		} else {
			batchResults, logText, err := r.runProbeBatchIsolated(batch)
			if err != nil {
				return nil, nil, fmt.Errorf("probe isolated batch %v: %w", batch, err)
			}
			for _, result := range batchResults {
				results[result.ProbeID] = result
			}
			r.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "isolated_batch", Log: logText})
		}
		timings = append(timings, ProbeTiming{Phase: "golden", ProbeIDs: append([]string(nil), batch...), Mode: "isolated_batch", DurationMS: time.Since(start).Milliseconds()})
		i += len(batch)
	}
	return results, timings, nil
}

func (r *RestExecutor) ensureAuth() error {
	if r.instanceURL != "" && r.accessToken != "" && r.client != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeRunTimeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sf", "org", "display", "--target-org", r.OrgAlias, "--verbose", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sf org display timed out after %s", probeRunTimeout())
		}
		return fmt.Errorf("sf org display failed: %w (stderr: %s)", err, stderr.String())
	}
	type orgResult struct {
		AccessToken string `json:"accessToken"`
		InstanceURL string `json:"instanceUrl"`
	}
	type orgOutput struct {
		Result orgResult `json:"result"`
	}
	var out orgOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return fmt.Errorf("parse sf org display output: %w", err)
	}
	if strings.TrimSpace(out.Result.AccessToken) == "" || strings.TrimSpace(out.Result.InstanceURL) == "" {
		return fmt.Errorf("sf org display returned empty access token or instance url")
	}
	r.accessToken = out.Result.AccessToken
	r.instanceURL = strings.TrimRight(out.Result.InstanceURL, "/")
	r.client = &http.Client{Timeout: probeRunTimeout()}
	return nil
}

func (r *RestExecutor) preflightShape() (map[string]interface{}, error) {
	jsonStr, _, err := r.runProbeCodeJSON("System.assert(false, 'GLADE_PROBE:' + ProbeRunner.preflight());")
	if err != nil {
		return nil, err
	}
	var shape map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &shape); err != nil {
		return nil, fmt.Errorf("parse preflight shape %q: %w", jsonStr, err)
	}
	return shape, nil
}

func (r *RestExecutor) runProbe(probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.assert(false, 'GLADE_PROBE:' + ProbeRunner.run('%s'));", probeID)
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
	jsonStr, logs, err := r.runProbeCodeJSON(code)
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
	if r.CaptureDebugLog && logs != "" {
		r.appendDebugLog(ProbeDebugLog{Phase: "golden", ProbeID: probeID, Mode: "single", Log: logs})
	}
	return result, nil
}

func (r *RestExecutor) runProbeBatch(probeIDs []string) ([]ProbeResult, string, error) {
	code := fmt.Sprintf("System.assert(false, 'GLADE_PROBE:' + ProbeRunner.runMany(new List<String>{%s}));", apexStringList(probeIDs))
	jsonStr, logs, err := r.runProbeCodeJSON(code)
	if err != nil {
		if strings.Contains(logs, "daily usage limit of apex log headers") {
			return nil, logs, fmt.Errorf("assertion batch unexpectedly requested debug logs: %w", err)
		}
		return nil, logs, err
	}
	var results []ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, logs, fmt.Errorf("parse probe batch result %q: %w", jsonStr, err)
	}
	return results, logs, nil
}

func (r *RestExecutor) runProbeBatchIsolated(probeIDs []string) ([]ProbeResult, string, error) {
	code := fmt.Sprintf("System.assert(false, 'GLADE_PROBE:' + ProbeRunner.runManyIsolated(new List<String>{%s}));", apexStringList(probeIDs))
	jsonStr, logs, err := r.runProbeCodeJSON(code)
	if err != nil {
		if strings.Contains(logs, "daily usage limit of apex log headers") {
			return nil, logs, fmt.Errorf("assertion isolated batch unexpectedly requested debug logs: %w", err)
		}
		return nil, logs, err
	}
	var results []ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, logs, fmt.Errorf("parse isolated probe batch result %q: %w", jsonStr, err)
	}
	return results, logs, nil
}

func (r *RestExecutor) appendDebugLog(entry ProbeDebugLog) {
	if !r.CaptureDebugLog || strings.TrimSpace(entry.Log) == "" {
		return
	}
	r.DebugLogs = append(r.DebugLogs, entry)
}

func (r *RestExecutor) runProbeCodeJSON(code string) (string, string, error) {
	if err := r.ensureAuth(); err != nil {
		return "", "", err
	}
	endpoint := fmt.Sprintf("%s/services/data/v60.0/tooling/executeAnonymous/?anonymousBody=%s", r.instanceURL, url.QueryEscape(code))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", fmt.Errorf("build executeAnonymous request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("tooling executeAnonymous request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read executeAnonymous response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("executeAnonymous http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	type execResult struct {
		Compiled            bool   `json:"compiled"`
		Success             bool   `json:"success"`
		CompileProblem      string `json:"compileProblem"`
		ExceptionMessage    string `json:"exceptionMessage"`
		ExceptionStackTrace string `json:"exceptionStackTrace"`
		DebugLog            string `json:"debugLog"`
	}
	var out execResult
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", fmt.Errorf("parse executeAnonymous response: %w (raw: %s)", err, string(body))
	}
	if !out.Compiled {
		return "", out.DebugLog, &apexCompileError{Problem: out.CompileProblem}
	}
	if !out.Success {
		if jsonStr, ok := extractAssertionJSON(out.ExceptionMessage); ok {
			return jsonStr, out.DebugLog, nil
		}
		return "", out.DebugLog, fmt.Errorf("apex execution failed: %s", strings.TrimSpace(out.ExceptionMessage))
	}
	jsonStr, err := extractDebugJSON(out.DebugLog)
	if err != nil {
		return "", out.DebugLog, fmt.Errorf("extract debug JSON: %w", err)
	}
	return jsonStr, out.DebugLog, nil
}
