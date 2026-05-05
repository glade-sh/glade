package compat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/server"
	"github.com/open-aer/oaer/internal/sobject"
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
	case "server":
		return runServerFixture(fixture)
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

	if err := writeFixtureFiles(root, fixture); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	var apexFiles []string
	sch := schema.Schema{}
	buildProject := project.Project{Root: root}
	if len(fixture.Schema) > 0 {
		if err := writeSFDXProject(root, fixture.Project); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		proj, err := project.Load(root)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		loaded, err := schema.LoadProject(proj)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		apexFiles = proj.ApexFiles
		sch = loaded
		buildProject = proj
	} else {
		apexFiles = make([]string, 0, len(fixture.Source))
		for _, source := range fixture.Source {
			path, err := fixturePath(root, source.Path)
			if err != nil {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
			}
			apexFiles = append(apexFiles, path)
		}
		buildProject.ApexFiles = apexFiles
	}

	index := typesys.Build(buildProject, sch)
	result := sema.Analyze(index)
	payload := map[string]any{
		"ok":          !result.HasErrors(),
		"files":       len(apexFiles),
		"types":       result.Summary.Types,
		"diagnostics": result.Summary.Diagnostics,
	}
	if len(fixture.Schema) > 0 {
		payload["schemaObjects"] = len(sch.Objects)
	}
	if len(fixture.Project.PackageDirectories) > 0 {
		payload["packageDirectories"] = len(fixture.Project.PackageDirectories)
	}
	if fixture.Project.Namespace != "" {
		payload["namespace"] = fixture.Project.Namespace
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
	machine := vm.New(&stdout)
	if len(fixture.Schema) > 0 || len(fixture.SeedData) > 0 || fixture.Project.Namespace != "" {
		org, err := orgFromFixture(fixture)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		machine.SetOrg(&org)
	}
	if fixture.Command.LimitMode != "" {
		mode, err := fixtureLimitMode(fixture.Command.LimitMode)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		machine.SetLimitMode(mode)
	}
	if len(fixture.Source) > 0 {
		if err := registerFixtureSourceClasses(machine, fixture); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
	}
	result, err := machine.Execute(program)
	if err != nil {
		return compareError(fixture, err)
	}
	payload := map[string]any{"ok": true, "debug": result.Debug}
	return compareResult(fixture, payload, stdout.String())
}

func registerFixtureSourceClasses(machine *vm.VM, fixture Fixture) error {
	root, err := os.MkdirTemp("", "oaer-compat-exec-source-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err := writeSFDXProject(root, fixture.Project); err != nil {
		return err
	}
	if err := writeFixtureFiles(root, fixture); err != nil {
		return err
	}
	proj, err := project.Load(root)
	if err != nil {
		return err
	}
	loadedSchema := schema.Schema{}
	if len(fixture.Schema) > 0 {
		loadedSchema, err = schema.LoadProject(proj)
		if err != nil {
			return err
		}
	}
	index := typesys.Build(proj, loadedSchema)
	if index.HasErrors() {
		if len(index.Diagnostics) > 0 {
			return fmt.Errorf("exec fixture %q source registration failed: %s", fixture.Name, index.Diagnostics[0].Message)
		}
		return fmt.Errorf("exec fixture %q source registration failed", fixture.Name)
	}
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		class := vm.Class{
			Name:         typ.Name,
			SuperClass:   typ.SuperClass,
			Interfaces:   typ.Interfaces,
			Fields:       make(map[string]vm.Field),
			StaticFields: make(map[string]vm.Field),
			IsInterface:  typ.Kind == apexast.DeclarationInterface,
			IsTest:       typ.IsTest,
			Access:       fixtureAccess(typ.Modifiers),
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationField && member.Kind != apexast.DeclarationProperty {
				continue
			}
			field := vm.Field{
				Name:     member.Name,
				Type:     member.Type,
				Access:   fixtureAccess(member.Modifiers),
				Property: member.Kind == apexast.DeclarationProperty,
				Static:   fixtureHasModifier(member.Modifiers, "static"),
			}
			if field.Static {
				class.StaticFields[field.Name] = field
				class.StaticFieldOrder = append(class.StaticFieldOrder, field.Name)
			} else {
				class.Fields[field.Name] = field
				class.FieldOrder = append(class.FieldOrder, field.Name)
			}
		}
		if err := machine.RegisterClass(class); err != nil {
			return err
		}
	}
	return nil
}

func fixtureAccess(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "public", "private", "protected", "global":
			return strings.ToLower(modifier)
		}
	}
	return ""
}

func fixtureHasModifier(modifiers []string, want string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, want) {
			return true
		}
	}
	return false
}

func orgFromFixture(fixture Fixture) (storage.OrgState, error) {
	root, err := os.MkdirTemp("", "oaer-compat-exec-*")
	if err != nil {
		return storage.OrgState{}, err
	}
	defer os.RemoveAll(root)
	if err := writeSFDXProject(root, fixture.Project); err != nil {
		return storage.OrgState{}, err
	}
	if err := writeFixtureFiles(root, fixture); err != nil {
		return storage.OrgState{}, err
	}
	proj, err := project.Load(root)
	if err != nil {
		return storage.OrgState{}, err
	}
	loadedSchema, err := schema.LoadProject(proj)
	if err != nil {
		return storage.OrgState{}, err
	}
	org := storage.NewOrgState()
	org.APIVersion = proj.SourceAPIVersion
	org.Namespace = proj.Namespace
	registry := sobject.BuildDescribeRegistry(loadedSchema)
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	if len(fixture.SeedData) > 0 {
		if err := storage.ApplyFixture(&org, storageFixture(fixture)); err != nil {
			return storage.OrgState{}, err
		}
	}
	return org, nil
}

func runTestFixture(fixture Fixture) (RunResult, error) {
	root, err := os.MkdirTemp("", "oaer-compat-test-*")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer os.RemoveAll(root)
	if err := writeSFDXProject(root, fixture.Project); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	if err := writeFixtureFiles(root, fixture); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	proj, err := project.Load(root)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	sch, err := schema.LoadProject(proj)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	opts := apextest.Options{}
	if fixture.Command.LimitMode != "" {
		mode, err := fixtureLimitMode(fixture.Command.LimitMode)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		opts.LimitMode = mode
	}
	run := apextest.Run(typesys.Build(proj, sch), opts)
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

func writeSFDXProject(root string, cfg ProjectConfig) error {
	type sfdxPackageDirectory struct {
		Path    string `json:"path"`
		Default bool   `json:"default,omitempty"`
	}
	type sfdxProject struct {
		PackageDirectories []sfdxPackageDirectory `json:"packageDirectories"`
		Namespace          string                 `json:"namespace,omitempty"`
		SourceAPIVersion   string                 `json:"sourceApiVersion,omitempty"`
	}
	packages := make([]sfdxPackageDirectory, 0, len(cfg.PackageDirectories))
	for _, pkg := range cfg.PackageDirectories {
		path := pkg.Path
		if path == "" {
			path = "force-app"
		}
		packages = append(packages, sfdxPackageDirectory{Path: path, Default: pkg.Default})
	}
	if len(packages) == 0 {
		packages = []sfdxPackageDirectory{{Path: "force-app", Default: true}}
	}
	data, err := json.Marshal(sfdxProject{
		PackageDirectories: packages,
		Namespace:          cfg.Namespace,
		SourceAPIVersion:   cfg.SourceAPIVersion,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "sfdx-project.json"), data, 0o644)
}

func writeFixtureFiles(root string, fixture Fixture) error {
	for _, source := range fixture.Source {
		if err := writeFixtureFile(root, source.Path, source.Content); err != nil {
			return err
		}
	}
	for _, schema := range fixture.Schema {
		if err := writeFixtureFile(root, schema.Path, schema.Content); err != nil {
			return err
		}
	}
	return nil
}

func writeFixtureFile(root, relativePath, content string) error {
	path, err := fixturePath(root, relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func fixturePath(root, relativePath string) (string, error) {
	clean := filepath.Clean(relativePath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fixture path %q must stay inside project root", relativePath)
	}
	return filepath.Join(root, clean), nil
}

func fixtureLimitMode(raw string) (vm.LimitMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "permissive":
		return vm.LimitModePermissive, nil
	case "strict":
		return vm.LimitModeStrict, nil
	default:
		return "", fmt.Errorf("unsupported limit mode %q", raw)
	}
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
	imported := storage.NewOrgState()
	if err := storage.ApplyFixture(&imported, exported); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	importedSummary := storage.InspectOrg("", imported)
	if err := store.Reset(org); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	resetSummary, err := store.Inspect("")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	payload := map[string]any{
		"ok":                  true,
		"schemaVersion":       seedSummary.SchemaVersion,
		"seedRecords":         seedSummary.Records,
		"resetRecords":        resetSummary.Records,
		"seedAccountRows":     seedSummary.ByObject["Account"],
		"resetAccountRows":    resetSummary.ByObject["Account"],
		"users":               seedSummary.Users,
		"profiles":            seedSummary.Profiles,
		"permissions":         seedSummary.Permissions,
		"exportedObjects":     len(exported.Objects),
		"exportedSequences":   len(exported.IDSequences),
		"importedRecords":     importedSummary.Records,
		"importedUsers":       importedSummary.Users,
		"importedProfiles":    importedSummary.Profiles,
		"importedAccountRows": importedSummary.ByObject["Account"],
	}
	return compareResult(fixture, payload, "")
}

func runServerFixture(fixture Fixture) (RunResult, error) {
	if len(fixture.ServerRequests) == 0 {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("server fixture requires serverRequests")
	}
	root, err := os.MkdirTemp(".", ".oaer-compat-server-*")
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer os.RemoveAll(root)
	dbPath := filepath.Join(root, "oaer.db")
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	defer func() {
		_ = store.Close()
	}()
	if err := writeFixtureFiles(root, fixture); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	if len(fixture.Source) > 0 || len(fixture.Schema) > 0 {
		if err := writeSFDXProject(root, fixture.Project); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
	}

	org := serverFixtureOrg()
	if len(fixture.SeedData) > 0 {
		if err := storage.ApplyFixture(&org, storageFixture(fixture)); err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
	}
	if err := store.Save(org); err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	source := server.SourceMetadata{}
	if len(fixture.Source) > 0 || len(fixture.Schema) > 0 {
		if p, err := project.Load(root); err == nil {
			source, _ = server.NewSourceMetadataFromProject(p)
		}
	}
	handler := server.NewWithStoreAndSource(&org, store, source)
	if fixture.Command.LimitMode != "" {
		mode, err := fixtureLimitMode(fixture.Command.LimitMode)
		if err != nil {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
		}
		handler.LimitMode = mode
	}
	statuses := make([]int, 0, len(fixture.ServerRequests))
	for i, step := range fixture.ServerRequests {
		if step.Restart {
			if err := store.Close(); err != nil {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
			}
			store, err = storage.OpenSQLite(dbPath)
			if err != nil {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
			}
			restartedOrg, err := store.Load()
			if err != nil {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
			}
			handler = server.NewWithStoreAndSource(&restartedOrg, store, source)
			if fixture.Command.LimitMode != "" {
				mode, err := fixtureLimitMode(fixture.Command.LimitMode)
				if err != nil {
					return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
				}
				handler.LimitMode = mode
			}
		}
		req := httptest.NewRequest(step.Method, step.Path, strings.NewReader(step.Body))
		if step.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for name, value := range step.Headers {
			req.Header.Set(name, value)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		statuses = append(statuses, rec.Code)
		if rec.Code != step.Status {
			return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("fixture %q server request %d %q status mismatch: expected %d, got %d body=%s", fixture.Name, i, step.Name, step.Status, rec.Code, rec.Body.String())
		}
		for _, want := range step.Contains {
			if !strings.Contains(rec.Body.String(), want) {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("fixture %q server request %d %q body missing %q: %s", fixture.Name, i, step.Name, want, rec.Body.String())
			}
		}
		for _, blocked := range step.NotContains {
			if strings.Contains(rec.Body.String(), blocked) {
				return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, fmt.Errorf("fixture %q server request %d %q body unexpectedly contained %q: %s", fixture.Name, i, step.Name, blocked, rec.Body.String())
			}
		}
	}
	persisted, err := store.Load()
	if err != nil {
		return RunResult{Name: fixture.Name, Kind: fixture.Command.Kind}, err
	}
	summary := storage.InspectOrg("", persisted)
	payload := map[string]any{
		"ok":                true,
		"requests":          len(fixture.ServerRequests),
		"statuses":          statuses,
		"persistedObjects":  summary.Objects,
		"persistedRecords":  summary.Records,
		"persistedAccounts": summary.ByObject["Account"],
		"users":             summary.Users,
		"profiles":          summary.Profiles,
		"permissions":       summary.Permissions,
	}
	return compareResult(fixture, payload, "")
}

func serverFixtureOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = "61.0"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
				"Description":    {APIName: "Description", Type: storage.FieldString},
				"External_Id__c": {APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true},
				"Formula__c":     {APIName: "Formula__c", Type: storage.FieldCalculated},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
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
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
		errorType = runtimeErr.Type
		message = runtimeErr.Message
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
