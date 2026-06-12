package enterpriseassess

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

type SourceFile struct {
	Path string
	Text string
}

type PatternFinding struct {
	ID      string
	Title   string
	File    string
	Line    int
	Summary string
}

func ReviewPatterns(files []SourceFile) []PatternFinding {
	var findings []PatternFinding
	findings = append(findings, findEmptyCatches(files)...)
	findings = append(findings, findDuplicateMockIDs(files)...)
	findings = append(findings, findDebugSOQL(files)...)
	findings = append(findings, findAPIVersionDrift(files)...)
	return findings
}

func findEmptyCatches(files []SourceFile) []PatternFinding {
	re := regexp.MustCompile(`(?is)catch\s*\(\s*(?:Exception|[A-Za-z0-9_.]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*\{(.*?)\}`)
	var out []PatternFinding
	for _, file := range files {
		for _, match := range re.FindAllStringSubmatchIndex(file.Text, -1) {
			name := file.Text[match[2]:match[3]]
			body := file.Text[match[4]:match[5]]
			bodyText := stripComments(body)
			if !regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(bodyText) {
				out = append(out, PatternFinding{
					ID:      "enterprise.review.empty_catch",
					Title:   "Empty catch swallows exception context",
					File:    file.Path,
					Line:    lineForOffset(file.Text, match[0]),
					Summary: "Catch block does not use the caught exception.",
				})
			}
		}
	}
	return out
}

func findDuplicateMockIDs(files []SourceFile) []PatternFinding {
	re := regexp.MustCompile(`(?is)\bmockId\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	seen := map[string]PatternFinding{}
	var out []PatternFinding
	for _, file := range files {
		for _, match := range re.FindAllStringSubmatchIndex(file.Text, -1) {
			value := file.Text[match[2]:match[3]]
			finding := PatternFinding{
				ID:      "enterprise.review.duplicate_mock_id",
				Title:   "Duplicate mockId value",
				File:    file.Path,
				Line:    lineForOffset(file.Text, match[0]),
				Summary: "mockId value " + value + " appears more than once.",
			}
			if _, ok := seen[value]; ok {
				out = append(out, finding)
				continue
			}
			seen[value] = finding
		}
	}
	return out
}

func findDebugSOQL(files []SourceFile) []PatternFinding {
	re := regexp.MustCompile(`(?is)\bSystem\s*\.\s*debug\s*\([^)]*\[\s*SELECT\b`)
	var out []PatternFinding
	for _, file := range files {
		for _, match := range re.FindAllStringIndex(file.Text, -1) {
			out = append(out, PatternFinding{
				ID:      "enterprise.review.debug_soql",
				Title:   "System.debug executes inline SOQL",
				File:    file.Path,
				Line:    lineForOffset(file.Text, match[0]),
				Summary: "Debug statement consumes a SOQL governor query.",
			})
		}
	}
	return out
}

func findAPIVersionDrift(files []SourceFile) []PatternFinding {
	projectVersion := ""
	for _, file := range files {
		if filepath.Base(file.Path) != "sfdx-project.json" {
			continue
		}
		var payload struct {
			SourceAPIVersion string `json:"sourceApiVersion"`
		}
		if json.Unmarshal([]byte(file.Text), &payload) == nil {
			projectVersion = payload.SourceAPIVersion
		}
	}
	if projectVersion == "" {
		return nil
	}
	re := regexp.MustCompile(`(?is)<apiVersion>\s*([^<]+)\s*</apiVersion>`)
	var out []PatternFinding
	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".cls-meta.xml") {
			continue
		}
		for _, match := range re.FindAllStringSubmatchIndex(file.Text, -1) {
			version := strings.TrimSpace(file.Text[match[2]:match[3]])
			if version == projectVersion {
				continue
			}
			out = append(out, PatternFinding{
				ID:      "enterprise.review.api_version_drift",
				Title:   "Apex metadata API version differs from project default",
				File:    file.Path,
				Line:    lineForOffset(file.Text, match[0]),
				Summary: "Class API version " + version + " differs from project sourceApiVersion " + projectVersion + ".",
			})
		}
	}
	return out
}

func stripComments(text string) string {
	block := regexp.MustCompile(`(?is)/\*.*?\*/`)
	line := regexp.MustCompile(`(?m)//.*$`)
	return line.ReplaceAllString(block.ReplaceAllString(text, ""), "")
}

func lineForOffset(text string, offset int) int {
	if offset < 0 {
		return 1
	}
	return strings.Count(text[:offset], "\n") + 1
}
