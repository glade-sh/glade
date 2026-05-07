package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ToolingSnippetSchemaVersion = 1

type ToolingSnippet struct {
	ID       string `json:"id"`
	Source   string `json:"source,omitempty"`
	File     string `json:"file,omitempty"`
	Category string `json:"category,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type ToolingSnippetReport struct {
	SchemaVersion int                    `json:"schemaVersion"`
	OrgAlias      string                 `json:"orgAlias,omitempty"`
	GeneratedAt   string                 `json:"generatedAtUtc,omitempty"`
	Snippets      []ToolingSnippetResult `json:"snippets"`
}

type ToolingSnippetResult struct {
	ID               string                     `json:"id"`
	Category         string                     `json:"category,omitempty"`
	Source           string                     `json:"source"`
	CLI              string                     `json:"cli"`
	Status           int                        `json:"status"`
	Compiled         bool                       `json:"compiled"`
	Executed         bool                       `json:"executed"`
	Success          bool                       `json:"success"`
	Line             int                        `json:"line,omitempty"`
	Column           int                        `json:"column,omitempty"`
	CompileProblem   string                     `json:"compileProblem,omitempty"`
	ExceptionType    string                     `json:"exceptionType,omitempty"`
	ExceptionMessage string                     `json:"exceptionMessage,omitempty"`
	LogsCaptured     bool                       `json:"logsCaptured"`
	RawShape         ToolingSnippetRawShape     `json:"rawShape"`
	Fixture          *ToolingSnippetFixture     `json:"fixture,omitempty"`
	Diagnostics      []ToolingSnippetDiagnostic `json:"diagnostics,omitempty"`
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

type toolingApexRunOutput struct {
	Status int                `json:"status"`
	Result toolingApexRunBody `json:"result"`
	Data   toolingApexRunBody `json:"data"`
}

type toolingApexRunBody struct {
	Success             bool    `json:"success"`
	Compiled            bool    `json:"compiled"`
	Executed            bool    `json:"executed"`
	Line                flexInt `json:"line"`
	Column              flexInt `json:"column"`
	CompileProblem      string  `json:"compileProblem"`
	ExceptionMessage    string  `json:"exceptionMessage"`
	ExceptionStackTrace string  `json:"exceptionStackTrace"`
	Logs                string  `json:"logs"`
}

func (s *SFDXExecutor) CaptureToolingSnippets(probeDir string, snippets []ToolingSnippet) (ToolingSnippetReport, error) {
	report := ToolingSnippetReport{
		SchemaVersion: ToolingSnippetSchemaVersion,
		OrgAlias:      s.OrgAlias,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Snippets:      make([]ToolingSnippetResult, 0, len(snippets)),
	}
	for _, snippet := range snippets {
		result, err := s.CaptureToolingSnippet(probeDir, snippet)
		if err != nil {
			return report, err
		}
		report.Snippets = append(report.Snippets, result)
	}
	return report, nil
}

func ReadToolingSnippetManifest(path string) ([]ToolingSnippet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapped struct {
		Snippets []ToolingSnippet `json:"snippets"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Snippets) > 0 {
		return wrapped.Snippets, nil
	}
	var snippets []ToolingSnippet
	if err := json.Unmarshal(data, &snippets); err != nil {
		return nil, fmt.Errorf("read Tooling snippet manifest %s: %w", path, err)
	}
	return snippets, nil
}

func WriteToolingSnippetReport(path string, report ToolingSnippetReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
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

func (s *SFDXExecutor) CaptureToolingSnippet(probeDir string, snippet ToolingSnippet) (ToolingSnippetResult, error) {
	source, err := snippetSource(snippet)
	if err != nil {
		return ToolingSnippetResult{}, err
	}
	outputBytes, cli, err := s.runWithSF(probeDir, source)
	if err != nil {
		outputBytes, cli, err = s.runWithSFDX(probeDir, source)
		if err != nil {
			return ToolingSnippetResult{}, err
		}
	}
	return ParseToolingSnippetOutput(snippet.ID, snippet.Category, source, cli, outputBytes)
}

func ParseToolingSnippetOutput(id, category, source, cli string, outputBytes []byte) (ToolingSnippetResult, error) {
	var output toolingApexRunOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return ToolingSnippetResult{}, fmt.Errorf("parse %s Tooling snippet output: %w", cli, err)
	}
	body, payloadKey := output.Result, "result"
	if isZeroToolingApexRunBody(body) && !isZeroToolingApexRunBody(output.Data) {
		body, payloadKey = output.Data, "data"
	}
	result := ToolingSnippetResult{
		ID:               id,
		Category:         category,
		Source:           source,
		CLI:              cli,
		Status:           output.Status,
		Compiled:         body.Compiled,
		Executed:         body.Executed,
		Success:          body.Success,
		Line:             int(body.Line),
		Column:           int(body.Column),
		CompileProblem:   body.CompileProblem,
		ExceptionMessage: body.ExceptionMessage,
		LogsCaptured:     body.Logs != "",
		RawShape:         toolingSnippetRawShape(outputBytes, payloadKey),
		Fixture:          &ToolingSnippetFixture{CommandKind: "tooling-execute-anonymous", Compiled: body.Compiled, Executed: body.Executed, Success: body.Success},
	}
	result.ExceptionType = exceptionTypeFromMessage(body.ExceptionMessage)
	if body.CompileProblem != "" {
		result.Diagnostics = append(result.Diagnostics, ToolingSnippetDiagnostic{Line: int(body.Line), Column: int(body.Column), Message: body.CompileProblem, Category: "compile"})
	}
	if body.ExceptionMessage != "" {
		result.Diagnostics = append(result.Diagnostics, ToolingSnippetDiagnostic{Message: body.ExceptionMessage, Category: "runtime"})
	}
	return result, nil
}

func snippetSource(snippet ToolingSnippet) (string, error) {
	if snippet.Source != "" && snippet.File != "" {
		return "", fmt.Errorf("snippet %q: use only one of source or file", snippet.ID)
	}
	if snippet.Source != "" {
		return snippet.Source, nil
	}
	if snippet.File == "" {
		return "", fmt.Errorf("snippet %q: source or file is required", snippet.ID)
	}
	data, err := os.ReadFile(snippet.File)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func isZeroToolingApexRunBody(body toolingApexRunBody) bool {
	return !body.Success && !body.Compiled && !body.Executed && body.Line == 0 && body.Column == 0 && body.CompileProblem == "" && body.ExceptionMessage == "" && body.Logs == ""
}

type flexInt int

func (i *flexInt) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case float64:
		*i = flexInt(v)
	case string:
		if v == "" {
			*i = 0
			return nil
		}
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		*i = flexInt(parsed)
	case nil:
		*i = 0
	default:
		return fmt.Errorf("unsupported integer value %T", raw)
	}
	return nil
}

func toolingSnippetRawShape(data []byte, payloadKey string) ToolingSnippetRawShape {
	var top map[string]json.RawMessage
	_ = json.Unmarshal(data, &top)
	shape := ToolingSnippetRawShape{TopLevelKeys: sortedMapKeys(top), PayloadKey: payloadKey}
	if payload, ok := top[payloadKey]; ok {
		var nested map[string]json.RawMessage
		_ = json.Unmarshal(payload, &nested)
		shape.ResultKeys = sortedMapKeys(nested)
	}
	return shape
}

var apexExceptionTypePattern = regexp.MustCompile(`^(?:System\.)?([A-Za-z][A-Za-z0-9_]*(?:Exception|Error)):`)

func exceptionTypeFromMessage(message string) string {
	message = strings.TrimSpace(message)
	matches := apexExceptionTypePattern.FindStringSubmatch(message)
	if len(matches) == 2 {
		if strings.HasPrefix(matches[1], "System.") {
			return matches[1]
		}
		return "System." + matches[1]
	}
	return ""
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
