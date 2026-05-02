package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/vm"
)

type RunResult struct {
	Name   string          `json:"name"`
	OK     bool            `json:"ok"`
	Kind   string          `json:"kind"`
	Result json.RawMessage `json:"result,omitempty"`
	Stdout string          `json:"stdout,omitempty"`
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
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	var stdout bytes.Buffer
	result, err := vm.Execute(program, &stdout)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	payload := map[string]any{"ok": true, "debug": result.Debug}
	return compareResult(fixture, payload, stdout.String())
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
