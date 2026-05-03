package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/typesys"
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
	case "check":
		return runCheckFixture(fixture)
	case "exec":
		return runExecFixture(fixture)
	case "test":
		return runTestFixture(fixture)
	case "db":
		return runDBFixture(fixture)
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
	diagnostics := 0
	for _, file := range result.Files {
		diagnostics += len(file.Diagnostics)
	}
	payload := map[string]any{"ok": !result.HasErrors(), "files": len(result.Files), "diagnostics": diagnostics}
	return compareResult(fixture, payload, "")
}

func runCheckFixture(fixture Fixture) (RunResult, error) {
	root, err := os.MkdirTemp("", "oaer-compat-check-*")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer os.RemoveAll(root)

	apexFiles := make([]string, 0, len(fixture.Source))
	for _, source := range fixture.Source {
		path := filepath.Join(root, filepath.Clean(source.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		if err := os.WriteFile(path, []byte(source.Content), 0o644); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		apexFiles = append(apexFiles, path)
	}

	index := typesys.Build(project.Project{Root: root, ApexFiles: apexFiles}, schema.Schema{})
	result := sema.Analyze(index)
	payload := map[string]any{
		"ok":          !result.HasErrors(),
		"files":       len(apexFiles),
		"types":       result.Summary.Types,
		"diagnostics": result.Summary.Diagnostics,
	}
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

func runTestFixture(fixture Fixture) (RunResult, error) {
	root, err := os.MkdirTemp("", "oaer-compat-test-*")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer os.RemoveAll(root)
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	for _, source := range fixture.Source {
		path := filepath.Join(root, filepath.Clean(source.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		if err := os.WriteFile(path, []byte(source.Content), 0o644); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
	}
	proj, err := project.Load(root)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	sch, err := schema.LoadProject(proj)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	run := apextest.Run(typesys.Build(proj, sch), apextest.Options{})
	summary := run.Summary()
	payload := map[string]any{
		"ok":     summary.Total > 0 && summary.Failed == 0 && summary.Errors == 0,
		"total":  summary.Total,
		"passed": summary.Passed,
		"failed": summary.Failed,
		"errors": summary.Errors,
	}
	return compareResult(fixture, payload, "")
}

func runDBFixture(fixture Fixture) (RunResult, error) {
	root, err := os.MkdirTemp("", "oaer-compat-db-*")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer os.RemoveAll(root)
	store, err := storage.OpenSQLite(filepath.Join(root, "oaer.db"))
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer store.Close()

	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	if err := storage.ApplyFixture(&org, storageFixture(fixture)); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	if err := store.Save(org); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	seedSummary, err := store.Inspect("")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	exported := storage.FixtureFromOrg(org)
	if err := store.Reset(org); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	resetSummary, err := store.Inspect("")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	payload := map[string]any{
		"ok":                true,
		"schemaVersion":     seedSummary.SchemaVersion,
		"seedRecords":       seedSummary.Records,
		"resetRecords":      resetSummary.Records,
		"seedAccountRows":   seedSummary.ByObject["Account"],
		"resetAccountRows":  resetSummary.ByObject["Account"],
		"users":             seedSummary.Users,
		"profiles":          seedSummary.Profiles,
		"permissions":       seedSummary.Permissions,
		"exportedObjects":   len(exported.Objects),
		"exportedSequences": len(exported.IDSequences),
	}
	return compareResult(fixture, payload, "")
}

func storageFixture(fixture Fixture) storage.Fixture {
	out := storage.NewFixture()
	for _, seed := range fixture.SeedData {
		object := storage.FixtureObject{Name: seed.Object}
		for _, record := range seed.Records {
			fields := make(map[string]storage.Value, len(record))
			for field, raw := range record {
				fields[field] = storageValue(raw)
			}
			object.Records = append(object.Records, storage.FixtureRecord{Fields: fields})
		}
		out.Objects = append(out.Objects, object)
	}
	return out
}

func storageValue(raw any) storage.Value {
	switch value := raw.(type) {
	case nil:
		return storage.NullValue()
	case string:
		return storage.StringValue(value)
	case bool:
		return storage.BooleanValue(value)
	case float64:
		if value == float64(int64(value)) {
			return storage.IntegerValue(int64(value))
		}
		return storage.DecimalValue(fmt.Sprintf("%g", value))
	default:
		return storage.StringValue(fmt.Sprint(value))
	}
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
