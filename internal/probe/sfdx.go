package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	for _, id := range probeIDs {
		result, err := s.runProbe(probeDir, id)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", id, err)
		}
		results[id] = result
	}
	return results, nil
}

func (s *SFDXExecutor) runProbe(probeDir, probeID string) (ProbeResult, error) {
	code := fmt.Sprintf("System.debug(ProbeRunner.run('%s'));", probeID)

	// Try modern sf CLI first (supports --code inline), then fall back to legacy sfdx.
	outputBytes, usedCmd, err := s.runWithSF(probeDir, code)
	if err != nil {
		outputBytes, usedCmd, err = s.runWithSFDX(probeDir, code)
		if err != nil {
			return ProbeResult{}, err
		}
	}

	type sfdxOutput struct {
		Status int `json:"status"`
		Result struct {
			Success  bool   `json:"success"`
			Compiled bool   `json:"compiled"`
			Executed bool   `json:"executed"`
			Logs     string `json:"logs"`
		} `json:"result"`
	}

	var output sfdxOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return ProbeResult{}, fmt.Errorf("parse %s output: %w (raw: %s)", usedCmd, err, string(outputBytes))
	}

	if !output.Result.Success {
		return ProbeResult{}, fmt.Errorf("apex execution failed via %s: compiled=%v executed=%v logs=%q", usedCmd, output.Result.Compiled, output.Result.Executed, output.Result.Logs)
	}

	jsonStr, err := extractDebugJSON(output.Result.Logs)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("extract debug JSON from %s: %w (raw logs: %q)", usedCmd, err, output.Result.Logs)
	}

	var result ProbeResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ProbeResult{}, fmt.Errorf("parse probe result %q: %w", jsonStr, err)
	}

	return result, nil
}

// runWithSF uses the modern "sf" CLI which supports inline --code.
func (s *SFDXExecutor) runWithSF(probeDir, code string) ([]byte, string, error) {
	cmd := exec.Command("sf", "apex", "run", "--target-org", s.OrgAlias, "--code", code, "--json")
	cmd.Dir = probeDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, "sf", fmt.Errorf("sf execution failed: %w (stderr: %s)", err, stderr.String())
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
		return nil, "sfdx", fmt.Errorf("sfdx execution failed: %w (stderr: %s)", err, stderr.String())
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
