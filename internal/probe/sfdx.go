package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// SFDXExecutor captures golden responses by running probes against a real
// Salesforce org via the sfdx/sf CLI.
type SFDXExecutor struct {
	OrgAlias string
}

// CaptureGolden runs each probe ID against the scratch org and returns the
// parsed responses.
func (s *SFDXExecutor) CaptureGolden(probeDir string, probeIDs []string) (map[string]ProbeResult, error) {
	results := make(map[string]ProbeResult, len(probeIDs))
	deployer := NewDeployer(probeDir, s.OrgAlias)
	if err := deployer.ResetProbeData(context.Background(), io.Discard); err != nil {
		return nil, fmt.Errorf("reset probe data before golden capture: %w", err)
	}
	for i, id := range probeIDs {
		fmt.Printf("  golden %d/%d %s\n", i+1, len(probeIDs), id)
		if isStatefulGoldenProbe(id) {
			if err := deployer.ResetProbeData(context.Background(), io.Discard); err != nil {
				return nil, fmt.Errorf("reset probe data before %s: %w", id, err)
			}
		}
		result, err := s.runProbe(probeDir, id)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", id, err)
		}
		results[id] = result
	}
	return results, nil
}

func isStatefulGoldenProbe(probeID string) bool {
	return strings.HasPrefix(probeID, "dml.") ||
		strings.HasPrefix(probeID, "bulkdml.") ||
		probeID == "limits.dml-rows"
}

func (s *SFDXExecutor) runProbe(probeDir, probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.debug(ProbeRunner.run('%s'));", probeID)
	result, logs, err := s.runProbeCode(probeDir, code)
	if err == nil {
		return result, nil
	}
	if strings.Contains(logs, "daily usage limit of apex log headers") {
		return s.runProbeViaAssertion(probeDir, probeID)
	}
	return ProbeResult{}, err
}

func (s *SFDXExecutor) runProbeViaAssertion(probeDir, probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.assert(false, 'OAER_PROBE:' + ProbeRunner.run('%s'));", probeID)
	result, _, err := s.runProbeCode(probeDir, code)
	return result, err
}

func (s *SFDXExecutor) runProbeCode(probeDir, code string) (ProbeResult, string, error) {
	// Try modern sf CLI first (supports --code inline), then fall back to legacy sfdx.
	outputBytes, usedCmd, err := s.runWithSF(probeDir, code)
	if err != nil {
		outputBytes, usedCmd, err = s.runWithSFDX(probeDir, code)
		if err != nil {
			return ProbeResult{}, "", err
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
		ExceptionMessage string `json:"exceptionMessage"`
	}
	type sfdxOutput struct {
		Status int        `json:"status"`
		Result sfdxResult `json:"result"`
		Data   sfdxResult `json:"data"`
	}

	var output sfdxOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return ProbeResult{}, "", fmt.Errorf("parse %s output: %w (raw: %s)", usedCmd, err, string(outputBytes))
	}

	r := output.Result
	if r.Logs == "" && !r.Success {
		r = output.Data
	}

	if !r.Success {
		if result, ok, err := extractAssertionProbeResult(r.ExceptionMessage); ok || err != nil {
			return result, r.Logs, err
		}
		return ProbeResult{}, r.Logs, fmt.Errorf("apex execution failed via %s: compiled=%v executed=%v logs=%q exc=%q", usedCmd, r.Compiled, r.Executed, r.Logs, r.ExceptionMessage)
	}

	jsonStr, err := extractDebugJSON(r.Logs)
	if err != nil {
		return ProbeResult{}, r.Logs, fmt.Errorf("extract debug JSON from %s: %w (raw logs: %q)", usedCmd, err, output.Result.Logs)
	}

	var result ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ProbeResult{}, r.Logs, fmt.Errorf("parse probe result %q: %w", jsonStr, err)
	}

	return result, r.Logs, nil
}

// runWithSF uses the modern "sf" CLI which requires a file path (-f).
func (s *SFDXExecutor) runWithSF(probeDir, code string) ([]byte, string, error) {
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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			return nil, "sf", fmt.Errorf("sf execution failed: %w (stderr: %s)", err, stderr.String())
		}
	}
	return stdout.Bytes(), "sf", nil
}

// runWithSFDX uses the legacy "sfdx" CLI which requires a file path (-f).
func (s *SFDXExecutor) runWithSFDX(probeDir, code string) ([]byte, string, error) {
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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			return nil, "sfdx", fmt.Errorf("sfdx execution failed: %w (stderr: %s)", err, stderr.String())
		}
	}
	return stdout.Bytes(), "sfdx", nil
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

func extractAssertionProbeResult(message string) (ProbeResult, bool, error) {
	const marker = "OAER_PROBE:"
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ProbeResult{}, false, nil
	}
	jsonStr := message[idx+len(marker):]
	var result ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ProbeResult{}, true, fmt.Errorf("parse assertion probe result %q: %w", jsonStr, err)
	}
	return result, true, nil
}
