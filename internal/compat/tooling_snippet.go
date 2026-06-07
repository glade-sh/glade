package compat

import (
	"encoding/json"
	"fmt"
	"os"
)

const ToolingSnippetSchemaVersion = 1

type ToolingSnippetReport struct {
	SchemaVersion int                    `json:"schemaVersion"`
	OrgAlias      string                 `json:"orgAlias,omitempty"`
	GeneratedAt   string                 `json:"generatedAtUtc,omitempty"`
	Snippets      []ToolingSnippetResult `json:"snippets"`
}

type ToolingSnippetResult struct {
	ID                  string                     `json:"id"`
	Category            string                     `json:"category,omitempty"`
	Source              string                     `json:"source"`
	CLI                 string                     `json:"cli"`
	Status              int                        `json:"status"`
	Compiled            bool                       `json:"compiled"`
	Executed            bool                       `json:"executed"`
	Success             bool                       `json:"success"`
	Line                int                        `json:"line,omitempty"`
	Column              int                        `json:"column,omitempty"`
	CompileProblem      string                     `json:"compileProblem,omitempty"`
	ExceptionType       string                     `json:"exceptionType,omitempty"`
	ExceptionMessage    string                     `json:"exceptionMessage,omitempty"`
	ExceptionStackTrace string                     `json:"exceptionStackTrace,omitempty"`
	LogsCaptured        bool                       `json:"logsCaptured"`
	RawLogs             string                     `json:"rawLogs,omitempty"`
	RawShape            ToolingSnippetRawShape     `json:"rawShape"`
	Fixture             *ToolingSnippetFixture     `json:"fixture,omitempty"`
	Diagnostics         []ToolingSnippetDiagnostic `json:"diagnostics,omitempty"`
}

type ToolingSnippetDiagnostic struct {
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Category string `json:"category,omitempty"`
}

type ToolingSnippetRawShape struct {
	TopLevelKeys []string `json:"topLevelKeys,omitempty"`
	PayloadKey   string   `json:"payloadKey,omitempty"`
	ResultKeys   []string `json:"resultKeys,omitempty"`
}

type ToolingSnippetFixture struct {
	CommandKind string `json:"commandKind"`
	Compiled    bool   `json:"compiled"`
	Executed    bool   `json:"executed"`
	Success     bool   `json:"success"`
}

func ReadToolingSnippetReport(path string) (ToolingSnippetReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolingSnippetReport{}, err
	}
	var report ToolingSnippetReport
	if err := json.Unmarshal(data, &report); err != nil {
		return ToolingSnippetReport{}, fmt.Errorf("read Tooling snippet report %s: %w", path, err)
	}
	return report, nil
}

func ValidateToolingSnippetReport(report ToolingSnippetReport) error {
	if report.SchemaVersion != ToolingSnippetSchemaVersion {
		return fmt.Errorf("tooling snippet report schemaVersion = %d, want %d", report.SchemaVersion, ToolingSnippetSchemaVersion)
	}
	if len(report.Snippets) == 0 {
		return fmt.Errorf("tooling snippet report requires at least one snippet")
	}
	for i, snippet := range report.Snippets {
		if snippet.ID == "" {
			return fmt.Errorf("tooling snippet report snippets[%d].id is required", i)
		}
		if snippet.Source == "" {
			return fmt.Errorf("tooling snippet report snippets[%d].source is required", i)
		}
		if snippet.CLI == "" {
			return fmt.Errorf("tooling snippet report snippets[%d].cli is required", i)
		}
		if snippet.RawShape.PayloadKey == "" {
			return fmt.Errorf("tooling snippet report snippets[%d].rawShape.payloadKey is required", i)
		}
		if snippet.Fixture == nil || snippet.Fixture.CommandKind != "tooling-execute-anonymous" {
			return fmt.Errorf("tooling snippet report snippets[%d].fixture.commandKind must be tooling-execute-anonymous", i)
		}
		if !snippet.Compiled && snippet.CompileProblem == "" {
			return fmt.Errorf("tooling snippet report snippets[%d] did not compile but has no compileProblem", i)
		}
		if snippet.ExceptionMessage != "" && len(snippet.Diagnostics) == 0 {
			return fmt.Errorf("tooling snippet report snippets[%d] has exceptionMessage but no diagnostics", i)
		}
	}
	return nil
}
