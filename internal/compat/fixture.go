package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Fixture struct {
	Name           string            `json:"name"`
	Evidence       []FixtureEvidence `json:"evidence,omitempty"`
	Project        ProjectConfig     `json:"project,omitempty"`
	Source         []SourceFile      `json:"source,omitempty"`
	Schema         []SchemaFile      `json:"schema,omitempty"`
	SeedData       []SeedData        `json:"seedData,omitempty"`
	ServerRequests []ServerRequest   `json:"serverRequests,omitempty"`
	Command        Invocation        `json:"command"`
	Expected       ExpectedBehavior  `json:"expected"`
	Limits         ExpectedLimits    `json:"limits,omitempty"`
}

type ProjectConfig struct {
	Namespace          string             `json:"namespace,omitempty"`
	SourceAPIVersion   string             `json:"sourceApiVersion,omitempty"`
	PackageDirectories []PackageDirectory `json:"packageDirectories,omitempty"`
}

type PackageDirectory struct {
	Path    string `json:"path"`
	Default bool   `json:"default,omitempty"`
}

type FixtureEvidence struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind,omitempty"`
	Notes  string `json:"notes,omitempty"`
}

type SourceFile struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type SchemaFile struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type SeedData struct {
	Object  string                   `json:"object"`
	Records []map[string]any         `json:"records"`
	Aliases map[string]RecordLocator `json:"aliases,omitempty"`
}

type RecordLocator struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}

type ServerRequest struct {
	Name     string            `json:"name,omitempty"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Status   int               `json:"status"`
	Contains []string          `json:"contains,omitempty"`
}

type Invocation struct {
	Kind      string   `json:"kind"`
	Args      []string `json:"args,omitempty"`
	LimitMode string   `json:"limitMode,omitempty"`
}

type ExpectedBehavior struct {
	Stdout      string           `json:"stdout,omitempty"`
	Stderr      string           `json:"stderr,omitempty"`
	Result      json.RawMessage  `json:"result,omitempty"`
	Error       *ExpectedError   `json:"error,omitempty"`
	SideEffects []ExpectedEffect `json:"sideEffects,omitempty"`
}

type ExpectedError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type ExpectedEffect struct {
	Object string         `json:"object"`
	ID     string         `json:"id,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

type ExpectedLimits struct {
	SOQLQueries   *int `json:"soqlQueries,omitempty"`
	SOQLRows      *int `json:"soqlRows,omitempty"`
	DMLStatements *int `json:"dmlStatements,omitempty"`
	DMLRows       *int `json:"dmlRows,omitempty"`
	CPUTimeMS     *int `json:"cpuTimeMs,omitempty"`
	HeapBytes     *int `json:"heapBytes,omitempty"`
}

func LoadFile(path string) (Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, err
	}
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

func SaveFile(path string, fixture Fixture) error {
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func Validate(fixture Fixture) error {
	if fixture.Name == "" {
		return errors.New("fixture name is required")
	}
	if fixture.Command.Kind == "" {
		return fmt.Errorf("fixture %q: command.kind is required", fixture.Name)
	}
	if len(fixture.Source) == 0 && len(fixture.Schema) == 0 && len(fixture.SeedData) == 0 && len(fixture.ServerRequests) == 0 {
		return fmt.Errorf("fixture %q: at least one source, schema, seed data, or server request entry is required", fixture.Name)
	}
	for i, source := range fixture.Source {
		if source.Path == "" {
			return fmt.Errorf("fixture %q: source[%d].path is required", fixture.Name, i)
		}
	}
	for i, evidence := range fixture.Evidence {
		if evidence.Symbol == "" {
			return fmt.Errorf("fixture %q: evidence[%d].symbol is required", fixture.Name, i)
		}
	}
	for i, schema := range fixture.Schema {
		if schema.Path == "" {
			return fmt.Errorf("fixture %q: schema[%d].path is required", fixture.Name, i)
		}
	}
	for i, seed := range fixture.SeedData {
		if seed.Object == "" {
			return fmt.Errorf("fixture %q: seedData[%d].object is required", fixture.Name, i)
		}
	}
	for i, request := range fixture.ServerRequests {
		if request.Method == "" {
			return fmt.Errorf("fixture %q: serverRequests[%d].method is required", fixture.Name, i)
		}
		if request.Path == "" {
			return fmt.Errorf("fixture %q: serverRequests[%d].path is required", fixture.Name, i)
		}
		if request.Status == 0 {
			return fmt.Errorf("fixture %q: serverRequests[%d].status is required", fixture.Name, i)
		}
	}
	return nil
}
