package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteReport writes a GapReport to disk as JSON.
func WriteReport(report *GapReport, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteFixtureStub generates a minimal compatibility fixture stub for a gap
// entry and writes it to the given directory.
func WriteFixtureStub(entry GapEntry, probeDir, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Determine which Apex class file contains the probe logic.
	className := guessClassName(entry.ProbeID)
	sourcePath := filepath.Join(probeDir, "force-app", "main", "default", "classes", className+".cls")
	code, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read probe source %s: %w", sourcePath, err)
	}

	// Build anonymous Apex that dispatches to the probe.
	anon := fmt.Sprintf("System.debug(ProbeRunner.run('%s'));", entry.ProbeID)

	fixture := map[string]interface{}{
		"name": entry.ProbeID,
		"source": []map[string]string{
			{"path": "classes/" + className + ".cls", "content": string(code)},
			{"path": "classes/Probe.cls", "content": readOrEmpty(filepath.Join(probeDir, "force-app", "main", "default", "classes", "Probe.cls"))},
			{"path": "classes/ProbeRunner.cls", "content": readOrEmpty(filepath.Join(probeDir, "force-app", "main", "default", "classes", "ProbeRunner.cls"))},
		},
		"command": map[string]interface{}{
			"kind": "exec",
			"args": []string{anon},
		},
		"expected": map[string]interface{}{
			"result": map[string]interface{}{
				"probeId": entry.ProbeID,
				"result":  entry.Golden,
			},
		},
	}

	path := filepath.Join(outDir, entry.ProbeID+".json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(fixture)
}

func guessClassName(probeID string) string {
	// probe IDs look like "stdlib.string.format-null" => class StdlibStringProbes
	if len(probeID) > 7 && probeID[:7] == "stdlib." {
		switch probeID[7:8] {
		case "s":
			return "StdlibStringProbes"
		case "d":
			return "StdlibDatetimeProbes"
		case "m":
			return "StdlibMathProbes"
		}
	}
	// Map category prefixes to probe class names
	if strings.HasPrefix(probeID, "async.") {
		return "AsyncApexProbes"
	}
	if strings.HasPrefix(probeID, "platform-event.") {
		return "PlatformEventProbes"
	}
	if strings.HasPrefix(probeID, "metadata.") {
		return "MetadataProbes"
	}
	if strings.HasPrefix(probeID, "integration.") {
		return "IntegrationProbes"
	}
	if strings.HasPrefix(probeID, "email.") {
		return "EmailProbes"
	}
	if strings.HasPrefix(probeID, "schema.") {
		return "SchemaDescribeProbes"
	}
	if strings.HasPrefix(probeID, "security.") {
		return "SecurityProbes"
	}
	return "ProbeRunner"
}

func readOrEmpty(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
