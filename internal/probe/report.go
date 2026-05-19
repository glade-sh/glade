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
	return writeIndentedJSON(path, report)
}

// WriteLocalRunReport writes local-only probe results to disk as JSON.
func WriteLocalRunReport(report *LocalRunReport, path string) error {
	return writeIndentedJSON(path, report)
}

// WriteManifest writes the selected probe manifest used by a run.
func WriteManifest(specs []ProbeSpec, path string) error {
	return writeIndentedJSON(path, specs)
}

func WriteGoldenCache(cache GoldenCache, path string) error {
	return writeIndentedJSON(path, cache)
}

func WriteDebugLogs(logs []ProbeDebugLog, path string) error {
	return writeIndentedJSON(path, logs)
}

func ReadGoldenCache(path string) (GoldenCache, error) {
	var cache GoldenCache
	f, err := os.Open(path)
	if err != nil {
		return cache, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cache); err != nil {
		return cache, err
	}
	return cache, nil
}

func AppendTrend(path string, entry TrendEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func writeIndentedJSON(path string, value any) error {
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
	return enc.Encode(value)
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
		return "AsyncProbes"
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
	if strings.HasPrefix(probeID, "bulkdml.") {
		return "BulkDmlProbes"
	}
	if strings.HasPrefix(probeID, "sobject.") || probeID == "id.validation" {
		return "SObjectProbes"
	}
	if strings.HasPrefix(probeID, "datetime.") {
		return "MoreDatetimeProbes"
	}
	if strings.HasPrefix(probeID, "string.") {
		return "MoreStringProbes"
	}
	if probeID == "soql.count-distinct" || probeID == "soql.group-by" || probeID == "soql.subquery" {
		return "MoreSoqlProbes"
	}
	if strings.HasPrefix(probeID, "soql.") {
		return "SoqlProbes"
	}
	if strings.HasPrefix(probeID, "math.") {
		return "MoreMathProbes"
	}
	if strings.HasPrefix(probeID, "system.") {
		return "SystemAssertProbes"
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
