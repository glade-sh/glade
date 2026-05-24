package compat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

const ReplaySchemaVersion = 1

type ReplayBundle struct {
	SchemaVersion int               `json:"schemaVersion"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Project       ReplayProject     `json:"project,omitempty"`
	Environment   ReplayEnvironment `json:"environment,omitempty"`
	Fixtures      ReplayFixtures    `json:"fixtures,omitempty"`
	Steps         []ReplayStep      `json:"steps"`
}

type ReplayProject struct {
	Root             string `json:"root,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"sourceApiVersion,omitempty"`
}

type ReplayEnvironment struct {
	Clock     string `json:"clock,omitempty"`
	UserID    string `json:"userId,omitempty"`
	LimitMode string `json:"limitMode,omitempty"`
}

type ReplayFixtures struct {
	Data     string `json:"data,omitempty"`
	Users    string `json:"users,omitempty"`
	Platform string `json:"platform,omitempty"`
}

type ReplayStep struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Args           []string        `json:"args,omitempty"`
	Expect         string          `json:"expect,omitempty"`
	ServerRequests []ServerRequest `json:"serverRequests,omitempty"`
}

type ReplayOptions struct {
	ContinueOnError bool
	ArtifactsDir    string
	CommandArgs     []string
}

type ReplayReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	Name          string             `json:"name"`
	OK            bool               `json:"ok"`
	Summary       ReplaySummary      `json:"summary"`
	Steps         []ReplayStepReport `json:"steps"`
}

type ReplaySummary struct {
	Steps       int `json:"steps"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	Unsupported int `json:"unsupported"`
	DurationMs  int `json:"durationMs"`
}

type ReplayStepReport struct {
	Name        string                  `json:"name"`
	Kind        string                  `json:"kind"`
	OK          bool                    `json:"ok"`
	DurationMs  int                     `json:"durationMs"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
	Blockers    []ReadinessBlocker      `json:"blockers,omitempty"`
	Result      json.RawMessage         `json:"result,omitempty"`
	Stdout      string                  `json:"stdout,omitempty"`
}

type ReplaySuiteReport struct {
	SchemaVersion int            `json:"schemaVersion"`
	OK            bool           `json:"ok"`
	Summary       ReplaySuiteSum `json:"summary"`
	Bundles       []ReplayReport `json:"bundles"`
}

type ReplaySuiteSum struct {
	Bundles    int `json:"bundles"`
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	DurationMs int `json:"durationMs"`
}

type ArtifactEnvironment struct {
	SchemaVersion int      `json:"schemaVersion"`
	Version       string   `json:"gladeVersion,omitempty"`
	GoVersion     string   `json:"goVersion"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	Args          []string `json:"args,omitempty"`
	WorkingDir    string   `json:"workingDirectory,omitempty"`
	Timestamp     string   `json:"timestamp"`
	APIVersion    string   `json:"apiVersion,omitempty"`
	LimitMode     string   `json:"limitMode,omitempty"`
	UserID        string   `json:"userId,omitempty"`
}

func LoadReplayBundle(root string) (ReplayBundle, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return ReplayBundle{}, err
	}
	data, err := os.ReadFile(filepath.Join(root, "replay.json"))
	if err != nil {
		return ReplayBundle{}, err
	}
	var bundle ReplayBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return ReplayBundle{}, err
	}
	if err := ValidateReplayBundle(root, bundle); err != nil {
		return ReplayBundle{}, err
	}
	return bundle, nil
}

func ValidateReplayBundle(root string, bundle ReplayBundle) error {
	if bundle.SchemaVersion == 0 {
		return errors.New("replay schemaVersion is required")
	}
	if bundle.SchemaVersion != ReplaySchemaVersion {
		return fmt.Errorf("unsupported replay schemaVersion %d", bundle.SchemaVersion)
	}
	if strings.TrimSpace(bundle.Name) == "" {
		return errors.New("replay name is required")
	}
	if bundle.Project.Root != "" {
		if _, err := replayPath(root, bundle.Project.Root); err != nil {
			return err
		}
	}
	if bundle.Environment.Clock != "" {
		if _, err := time.Parse(time.RFC3339, bundle.Environment.Clock); err != nil {
			return fmt.Errorf("replay environment.clock: %w", err)
		}
	}
	if bundle.Environment.LimitMode != "" {
		if _, err := fixtureLimitMode(bundle.Environment.LimitMode); err != nil {
			return err
		}
	}
	for label, path := range map[string]string{
		"fixtures.data":     bundle.Fixtures.Data,
		"fixtures.users":    bundle.Fixtures.Users,
		"fixtures.platform": bundle.Fixtures.Platform,
	} {
		if path == "" {
			continue
		}
		if _, err := replayPath(root, path); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	if len(bundle.Steps) == 0 {
		return fmt.Errorf("replay %q: at least one step is required", bundle.Name)
	}
	for i, step := range bundle.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return fmt.Errorf("replay %q: steps[%d].name is required", bundle.Name, i)
		}
		if strings.TrimSpace(step.Kind) == "" {
			return fmt.Errorf("replay %q: steps[%d].kind is required", bundle.Name, i)
		}
		if !supportedReplayKind(step.Kind) {
			return fmt.Errorf("replay %q: steps[%d].kind %q is unsupported", bundle.Name, i, step.Kind)
		}
		if step.Expect != "" {
			path, err := replayPath(root, step.Expect)
			if err != nil {
				return fmt.Errorf("replay %q: steps[%d].expect: %w", bundle.Name, i, err)
			}
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("replay %q: steps[%d].expect: %w", bundle.Name, i, err)
			}
		}
		for j, request := range step.ServerRequests {
			if request.Method == "" {
				return fmt.Errorf("replay %q: steps[%d].serverRequests[%d].method is required", bundle.Name, i, j)
			}
			if request.Path == "" {
				return fmt.Errorf("replay %q: steps[%d].serverRequests[%d].path is required", bundle.Name, i, j)
			}
			if request.Status == 0 {
				return fmt.Errorf("replay %q: steps[%d].serverRequests[%d].status is required", bundle.Name, i, j)
			}
		}
	}
	return nil
}

func RunReplayBundles(paths []string, opts ReplayOptions) (ReplaySuiteReport, error) {
	start := time.Now()
	suite := ReplaySuiteReport{SchemaVersion: ReplaySchemaVersion, OK: true}
	for _, path := range paths {
		report, err := RunReplayBundle(path, opts)
		if err != nil {
			return suite, err
		}
		suite.Bundles = append(suite.Bundles, report)
		if report.OK {
			suite.Summary.Passed++
		} else {
			suite.OK = false
			suite.Summary.Failed++
		}
	}
	suite.Summary.Bundles = len(suite.Bundles)
	suite.Summary.DurationMs = int(time.Since(start).Milliseconds())
	return suite, nil
}

func RunReplayBundle(root string, opts ReplayOptions) (ReplayReport, error) {
	start := time.Now()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ReplayReport{}, err
	}
	bundle, err := LoadReplayBundle(absRoot)
	if err != nil {
		return ReplayReport{}, err
	}
	report := ReplayReport{SchemaVersion: ReplaySchemaVersion, Name: bundle.Name, OK: true}
	for _, step := range bundle.Steps {
		stepReport := runReplayStep(absRoot, bundle, step)
		report.Steps = append(report.Steps, stepReport)
		if stepReport.OK {
			report.Summary.Passed++
		} else {
			report.OK = false
			report.Summary.Failed++
			report.Summary.Unsupported += countUnsupported(stepReport)
			if !opts.ContinueOnError {
				break
			}
		}
	}
	report.Summary.Steps = len(report.Steps)
	report.Summary.DurationMs = int(time.Since(start).Milliseconds())
	if opts.ArtifactsDir != "" {
		if err := WriteReplayArtifacts(opts.ArtifactsDir, absRoot, report, bundle, opts); err != nil {
			return report, err
		}
	}
	return report, nil
}

func WriteReplayJSON(w io.Writer, report any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteReplayText(w io.Writer, report ReplaySuiteReport) {
	if len(report.Bundles) == 1 {
		writeReplayBundleText(w, report.Bundles[0])
		return
	}
	state := "passed"
	if !report.OK {
		state = "blocked"
	}
	fmt.Fprintf(w, "Replay: %s\n", state)
	fmt.Fprintf(w, "Bundles: %d passed=%d failed=%d\n", report.Summary.Bundles, report.Summary.Passed, report.Summary.Failed)
	for _, bundle := range report.Bundles {
		status := "passed"
		if !bundle.OK {
			status = "blocked"
		}
		fmt.Fprintf(w, "  %s: %s steps=%d failed=%d\n", bundle.Name, status, bundle.Summary.Steps, bundle.Summary.Failed)
	}
}

func writeReplayBundleText(w io.Writer, report ReplayReport) {
	state := "passed"
	if !report.OK {
		state = "blocked"
	}
	fmt.Fprintf(w, "Replay %s: %s\n", report.Name, state)
	fmt.Fprintf(w, "Steps: %d passed=%d failed=%d unsupported=%d\n", report.Summary.Steps, report.Summary.Passed, report.Summary.Failed, report.Summary.Unsupported)
	for _, step := range report.Steps {
		status := "passed"
		if !step.OK {
			status = "failed"
		}
		fmt.Fprintf(w, "  %s [%s]: %s\n", step.Name, step.Kind, status)
		for _, blocker := range step.Blockers {
			fmt.Fprintf(w, "    - %s: %s\n", blocker.Category, blocker.Message)
		}
	}
}

func WriteReplayArtifacts(dir, bundleRoot string, report ReplayReport, bundle ReplayBundle, opts ReplayOptions) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var reportBuf bytes.Buffer
	if err := WriteReplayJSON(&reportBuf, report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), reportBuf.Bytes(), 0o644); err != nil {
		return err
	}
	env := ArtifactEnvironment{
		SchemaVersion: ReplaySchemaVersion,
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Args:          opts.CommandArgs,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		APIVersion:    bundle.Project.SourceAPIVersion,
		LimitMode:     bundle.Environment.LimitMode,
		UserID:        bundle.Environment.UserID,
	}
	if cwd, err := os.Getwd(); err == nil {
		env.WorkingDir = cwd
	}
	var envBuf bytes.Buffer
	if err := WriteReplayJSON(&envBuf, env); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "environment.json"), envBuf.Bytes(), 0o644); err != nil {
		return err
	}
	return copyReplayBundle(filepath.Join(dir, "bundle"), bundleRoot)
}

func runReplayStep(root string, bundle ReplayBundle, step ReplayStep) ReplayStepReport {
	start := time.Now()
	out := ReplayStepReport{Name: step.Name, Kind: step.Kind}
	fixture, err := replayFixture(root, bundle, step)
	if err == nil {
		var result RunResult
		result, err = Run(fixture)
		out.Result = result.Result
		out.Stdout = result.Stdout
		out.OK = result.OK && replayResultOK(result.Result)
	}
	if err == nil && step.Expect != "" {
		err = compareReplayExpected(root, step.Expect, out.Result)
		if err != nil {
			out.OK = false
		}
	}
	if err == nil && !out.OK {
		err = fmt.Errorf("replay step %q reported ok=false", step.Name)
	}
	if err != nil {
		diag := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEREPLAY001", Message: err.Error()}
		if isUnsupportedMessage(err.Error()) {
			diag.Code = "UNSUPPORTED_FEATURE"
		}
		out.Diagnostics = append(out.Diagnostics, diag)
		out.Blockers = append(out.Blockers, ClassifyReadinessDiagnostic(diag))
		out.OK = false
	}
	out.DurationMs = int(time.Since(start).Milliseconds())
	return out
}

func replayResultOK(result json.RawMessage) bool {
	if len(result) == 0 {
		return true
	}
	var payload struct {
		OK *bool `json:"ok"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.OK == nil {
		return true
	}
	return *payload.OK
}

func replayFixture(root string, bundle ReplayBundle, step ReplayStep) (Fixture, error) {
	projectRoot, err := replayProjectRoot(root, bundle.Project.Root)
	if err != nil {
		return Fixture{}, err
	}
	fixture := Fixture{
		Name: step.Name,
		Project: ProjectConfig{
			Namespace:        bundle.Project.Namespace,
			SourceAPIVersion: bundle.Project.SourceAPIVersion,
		},
		Command: Invocation{
			Kind:      step.Kind,
			Args:      step.Args,
			LimitMode: bundle.Environment.LimitMode,
		},
		ServerRequests: step.ServerRequests,
	}
	if err := loadReplayProjectConfig(projectRoot, &fixture.Project); err != nil {
		return Fixture{}, err
	}
	if err := materializeReplayFiles(root, projectRoot, &fixture); err != nil {
		return Fixture{}, err
	}
	if bundle.Fixtures.Data != "" {
		seed, err := loadReplaySeedData(root, bundle.Fixtures.Data)
		if err != nil {
			return Fixture{}, err
		}
		fixture.SeedData = seed
	}
	if fixture.Command.LimitMode == "" {
		fixture.Command.LimitMode = "permissive"
	}
	if step.Kind == "server" && len(fixture.ServerRequests) == 0 {
		return Fixture{}, fmt.Errorf("server replay step %q requires serverRequests", step.Name)
	}
	if step.Kind == "exec" && len(fixture.Command.Args) == 0 {
		return Fixture{}, fmt.Errorf("exec replay step %q requires args[0]", step.Name)
	}
	if step.Kind == "exec" && len(fixture.Source) == 0 {
		fixture.Source = append(fixture.Source, SourceFile{Path: "anonymous.apex", Content: fixture.Command.Args[0]})
	}
	return fixture, nil
}

func materializeReplayFiles(bundleRoot, projectRoot string, fixture *Fixture) error {
	return filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(bundleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		lower := strings.ToLower(path)
		switch {
		case strings.HasSuffix(lower, ".cls"), strings.HasSuffix(lower, ".trigger"):
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fixture.Source = append(fixture.Source, SourceFile{Path: rel, Content: string(content)})
		case strings.HasSuffix(lower, ".object-meta.xml"), strings.HasSuffix(lower, ".field-meta.xml"), strings.HasSuffix(lower, ".recordtype-meta.xml"), strings.HasSuffix(lower, ".validationrule-meta.xml"):
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fixture.Schema = append(fixture.Schema, SchemaFile{Path: rel, Content: string(content)})
		}
		return nil
	})
}

func loadReplayProjectConfig(root string, cfg *ProjectConfig) error {
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.PackageDirectories = []PackageDirectory{{Path: "force-app", Default: true}}
			return nil
		}
		return err
	}
	var raw struct {
		PackageDirectories []PackageDirectory `json:"packageDirectories"`
		Namespace          string             `json:"namespace"`
		SourceAPIVersion   string             `json:"sourceApiVersion"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cfg.PackageDirectories = raw.PackageDirectories
	if cfg.Namespace == "" {
		cfg.Namespace = raw.Namespace
	}
	if cfg.SourceAPIVersion == "" {
		cfg.SourceAPIVersion = raw.SourceAPIVersion
	}
	return nil
}

func loadReplaySeedData(root, relativePath string) ([]SeedData, error) {
	path, err := replayPath(root, relativePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var seed []SeedData
	if err := json.Unmarshal(data, &seed); err == nil {
		return seed, nil
	}
	var wrapped struct {
		SeedData []SeedData `json:"seedData"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.SeedData != nil {
		return wrapped.SeedData, nil
	}
	var fixture storage.Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("replay seed data must be compat seedData or storage fixture: %w", err)
	}
	return seedDataFromStorageFixture(fixture), nil
}

func seedDataFromStorageFixture(fixture storage.Fixture) []SeedData {
	out := make([]SeedData, 0, len(fixture.Objects))
	for _, object := range fixture.Objects {
		seed := SeedData{Object: object.Name}
		for _, record := range object.Records {
			fields := map[string]any{}
			if record.ID != "" {
				fields["Id"] = string(record.ID)
			}
			for field, value := range record.Fields {
				fields[field] = storageValueAny(value)
			}
			seed.Records = append(seed.Records, fields)
		}
		out = append(out, seed)
	}
	return out
}

func storageValueAny(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, storageValueAny(item))
		}
		return out
	default:
		return value.String
	}
}

func compareReplayExpected(root, relativePath string, actual json.RawMessage) error {
	path, err := replayPath(root, relativePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var expected any
	if err := json.Unmarshal(data, &expected); err != nil {
		return err
	}
	var got any
	if err := json.Unmarshal(actual, &got); err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, got) {
		return fmt.Errorf("expected %s mismatch: expected %s, got %s", relativePath, strings.TrimSpace(string(data)), string(actual))
	}
	return nil
}

func replayProjectRoot(root, projectRoot string) (string, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	return replayPath(root, projectRoot)
}

func replayPath(root, relativePath string) (string, error) {
	clean := filepath.Clean(relativePath)
	if clean == "." {
		return root, nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("replay path %q must stay inside bundle root", relativePath)
	}
	path := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("replay path %q must stay inside bundle root", relativePath)
	}
	return path, nil
}

func supportedReplayKind(kind string) bool {
	switch kind {
	case "parse", "check", "exec", "test", "db", "server":
		return true
	default:
		return false
	}
}

func countUnsupported(report ReplayStepReport) int {
	count := 0
	for _, blocker := range report.Blockers {
		if blocker.Category == "stdlib" || blocker.Code == "UNSUPPORTED_FEATURE" {
			count++
		}
	}
	return count
}

func isUnsupportedMessage(message string) bool {
	return strings.Contains(strings.ToLower(message), "unsupported")
}

func copyReplayBundle(dst, src string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("replay artifact path %q must not be a symlink", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, redactReplayArtifact(path, data), 0o644)
	})
}

func redactReplayArtifact(path string, data []byte) []byte {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return data
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return data
	}
	redactJSON(value)
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return data
	}
	return append(out, '\n')
}

func redactJSON(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if redactedKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactJSON(child)
		}
	case []any:
		for _, child := range typed {
			redactJSON(child)
		}
	}
}

func redactedKey(key string) bool {
	lower := strings.ToLower(key)
	return lower == "authorization" || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "key")
}
