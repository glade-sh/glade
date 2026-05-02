package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/vm"
)

type RunResult struct {
	Name   string          `json:"name"`
	OK     bool            `json:"ok"`
	Kind   string          `json:"kind"`
	Result json.RawMessage `json:"result,omitempty"`
	Stdout string          `json:"stdout,omitempty"`
	Error  *ExpectedError  `json:"error,omitempty"`
}

func Run(fixture Fixture) (RunResult, error) {
	if err := Validate(fixture); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}

	switch fixture.Command.Kind {
	case "parse":
		return runParseFixture(fixture)
	case "exec":
		return runExecFixture(fixture)
	default:
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("unsupported fixture command kind %q", fixture.Command.Kind)
	}
}

func runParseFixture(fixture Fixture) (RunResult, error) {
	parser := apexast.NewParser()
	result := apexast.Result{}
	for _, source := range fixture.Source {
		result.Files = append(result.Files, parser.ParseSource(source.Path, source.Content))
	}
	payload := map[string]any{"ok": !result.HasErrors(), "files": len(result.Files)}
	return compareResult(fixture, payload, "")
}

func runExecFixture(fixture Fixture) (RunResult, error) {
	if len(fixture.Command.Args) == 0 {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("exec fixture requires command.args[0]")
	}
	program, err := vm.CompileAnonymous(fixture.Command.Args[0])
	if err != nil {
		return compareError(fixture, err)
	}
	var stdout bytes.Buffer
	result, err := vm.Execute(program, &stdout)
	if err != nil {
		return compareError(fixture, err)
	}
	payload := map[string]any{"ok": true, "debug": result.Debug}
	return compareResult(fixture, payload, stdout.String())
}

func compareError(fixture Fixture, runErr error) (RunResult, error) {
	actual := classifyError(runErr)
	out := RunResult{
		Name:  fixture.Name,
		Kind:  fixture.Command.Kind,
		Error: &actual,
	}
	if fixture.Expected.Error == nil {
		return out, runErr
	}
	expected := *fixture.Expected.Error
	if expected.Type != "" && expected.Type != actual.Type {
		return out, fmt.Errorf("fixture %q error type mismatch: expected %q, got %q", fixture.Name, expected.Type, actual.Type)
	}
	if expected.Code != "" && expected.Code != actual.Code {
		return out, fmt.Errorf("fixture %q error code mismatch: expected %q, got %q", fixture.Name, expected.Code, actual.Code)
	}
	if expected.Message != "" && !strings.Contains(actual.Message, expected.Message) {
		return out, fmt.Errorf("fixture %q error message mismatch: expected to contain %q, got %q", fixture.Name, expected.Message, actual.Message)
	}
	out.OK = true
	payload := map[string]any{"ok": false, "error": actual}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	out.Result = encoded
	return out, nil
}

func classifyError(err error) ExpectedError {
	message := err.Error()
	errorType := "Error"
	if strings.Contains(strings.ToLower(message), "unsupported") {
		errorType = "UnsupportedFeature"
	}
	return ExpectedError{Type: errorType, Message: message}
}

func compareResult(fixture Fixture, payload map[string]any, stdout string) (RunResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	out := RunResult{
		Name:   fixture.Name,
		Kind:   fixture.Command.Kind,
		Result: encoded,
		Stdout: stdout,
	}
	if len(fixture.Expected.Result) > 0 {
		var expected any
		var actual any
		if err := json.Unmarshal(fixture.Expected.Result, &expected); err != nil {
			return out, err
		}
		if err := json.Unmarshal(encoded, &actual); err != nil {
			return out, err
		}
		if !reflect.DeepEqual(expected, actual) {
			return out, fmt.Errorf("fixture %q result mismatch: expected %s, got %s", fixture.Name, fixture.Expected.Result, encoded)
		}
	}
	if fixture.Expected.Stdout != "" && fixture.Expected.Stdout != stdout {
		return out, fmt.Errorf("fixture %q stdout mismatch: expected %q, got %q", fixture.Name, fixture.Expected.Stdout, stdout)
	}
	out.OK = true
	return out, nil
}
