package diagnostic

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	ShortDesc sarifMessage `json:"shortDescription,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId,omitempty"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation,omitempty"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

func (r Report) WriteSARIF(w io.Writer) error {
	rules := make([]sarifRule, 0)
	seenRules := make(map[string]bool)
	results := make([]sarifResult, 0, len(r.Diagnostics))
	for _, diag := range r.Diagnostics {
		if diag.Code != "" && !seenRules[diag.Code] {
			seenRules[diag.Code] = true
			rules = append(rules, sarifRule{
				ID:        diag.Code,
				Name:      diag.Code,
				ShortDesc: sarifMessage{Text: diag.Message},
			})
		}
		result := sarifResult{
			RuleID:  diag.Code,
			Level:   sarifLevel(diag.Severity),
			Message: sarifMessage{Text: diag.Message},
		}
		if diag.File != "" {
			location := sarifLocation{PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: filepathToURI(diag.File)},
			}}
			if diag.Range != nil {
				location.PhysicalLocation.Region = sarifRegion{
					StartLine:   diag.Range.Start.Line,
					StartColumn: diag.Range.Start.Column,
					EndLine:     diag.Range.End.Line,
					EndColumn:   diag.Range.End.Column,
				}
			}
			result.Locations = []sarifLocation{location}
		}
		results = append(results, result)
	}
	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "glade", Rules: rules}},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func (r Report) WriteGitHubAnnotations(w io.Writer) error {
	for _, diag := range r.Diagnostics {
		level := "notice"
		switch diag.Severity {
		case Error:
			level = "error"
		case Warning:
			level = "warning"
		}
		props := []string{}
		if diag.File != "" {
			props = append(props, "file="+escapeGitHubAnnotationProperty(diag.File))
		}
		if diag.Range != nil && diag.Range.Start.Line > 0 {
			props = append(props, fmt.Sprintf("line=%d", diag.Range.Start.Line))
			if diag.Range.Start.Column > 0 {
				props = append(props, fmt.Sprintf("col=%d", diag.Range.Start.Column))
			}
		}
		if diag.Code != "" {
			props = append(props, "title="+escapeGitHubAnnotationProperty(diag.Code))
		}
		command := "::" + level
		if len(props) > 0 {
			command += " " + strings.Join(props, ",")
		}
		if _, err := fmt.Fprintf(w, "%s::%s\n", command, escapeGitHubAnnotationData(diag.Message)); err != nil {
			return err
		}
	}
	return nil
}

func sarifLevel(severity Severity) string {
	switch severity {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "note"
	}
}

func filepathToURI(path string) string {
	return strings.ReplaceAll(path, `\`, `/`)
}

func escapeGitHubAnnotationData(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	return value
}

func escapeGitHubAnnotationProperty(value string) string {
	value = escapeGitHubAnnotationData(value)
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	return value
}
