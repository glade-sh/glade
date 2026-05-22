package playground

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

type vmOrgStore struct {
	mu  sync.Mutex
	org storage.OrgState
	db  string
}

type Runner struct {
	workspace          *Workspace
	version            string
	cache              *ResultCache
	store              *vmOrgStore
	loadWorkspaceIndex workspaceIndexLoader
	runtimeTemplate    *cachedRuntimeTemplate
	mu                 sync.Mutex
}

type workspaceIndexLoader func(string) (typesys.Index, []diagnostic.Diagnostic, error)

type cachedRuntimeTemplate struct {
	workspaceHash string
	projectRoot   string
	version       string
	template      *vm.VM
	diagnostics   []diagnostic.Diagnostic
}

func NewRunner(workspace *Workspace, opts RunnerOptions) *Runner {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	store := opts.Org
	if store == nil {
		store = &vmOrgStore{org: baselineOrg(), db: opts.DBPath}
		if opts.DBPath != "" {
			store.org = loadOrCreateDBOrg(opts.DBPath)
		}
	}
	return &Runner{
		workspace:          workspace,
		version:            version,
		cache:              NewResultCache(filepath.Join(workspace.DataRoot, "cache")),
		store:              store,
		loadWorkspaceIndex: loadWorkspaceIndex,
	}
}

func (r *Runner) Org() storage.OrgState {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.store.org.Clone()
}

func (r *Runner) Reset() {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.org = baselineOrg()
	_ = saveDBOrg(r.store.db, r.store.org)
}

func (r *Runner) Seed(reader io.Reader) error {
	fixture, err := storage.ReadFixture(reader)
	if err != nil {
		return err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	org := r.store.org.Clone()
	if len(org.Objects) == 0 {
		org = baselineOrg()
	}
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		return err
	}
	r.store.org = org
	return saveDBOrg(r.store.db, org)
}

func (r *Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	if req.Mode == "" {
		req.Mode = RunModeScratch
	}
	if req.LimitMode == "" {
		req.LimitMode = vm.LimitModePermissive
	}
	started := time.Now().UTC()
	workspaceHash, err := r.workspace.Hash()
	if err != nil {
		return RunResult{}, err
	}
	seedHash := fileHash(filepath.Join(r.workspace.Root, "seed.json"))
	cacheKey := CacheKey{
		WorkspaceHash: workspaceHash,
		AnonymousBody: req.AnonymousBody,
		SeedHash:      seedHash,
		ProjectRoot:   r.workspace.ProjectRoot,
		LimitMode:     string(req.LimitMode),
		RunMode:       string(req.Mode),
		Version:       r.version,
	}.String()
	if req.UseCache {
		if cached, ok, err := r.cache.Load(cacheKey); err != nil {
			return RunResult{}, err
		} else if ok {
			cached.CacheHit = true
			return cached, nil
		}
	}

	result := RunResult{
		RunID:     runID(started),
		Status:    RunStatusPass,
		LimitMode: req.LimitMode,
		CacheKey:  cacheKey,
		StartedAt: started,
	}

	compileStart := time.Now()
	template, indexDiagnostics, err := r.loadRuntimeTemplate(workspaceHash)
	if err != nil {
		result.Status = RunStatusCompileError
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: err.Error()})
		result.CompletedAt = time.Now().UTC()
		return result, nil
	}
	result.Diagnostics = append(result.Diagnostics, diagnosticsFromIndex(indexDiagnostics)...)

	program, err := vm.CompileAnonymous(req.AnonymousBody)
	result.CompileMS = millisSince(compileStart)
	if err != nil {
		result.Status = RunStatusCompileError
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: err.Error()})
		result.CompletedAt = time.Now().UTC()
		return result, nil
	}

	r.store.mu.Lock()
	before := r.store.org.Clone()
	runOrg := before.Clone()
	r.store.mu.Unlock()

	machine := template.CloneRuntime(nil)
	machine.SetContext(ctx)
	machine.SetTraceEnabled(true)
	machine.SetLimitMode(req.LimitMode)
	machine.SetOrg(&runOrg)

	execStart := time.Now()
	vmResult, execErr := machine.Execute(program)
	if execErr == nil {
		execErr = machine.DrainAsync(&vmResult)
	}
	result.ExecuteMS = millisSince(execStart)
	result.Logs = append([]string(nil), vmResult.Debug...)
	result.Limits = vmResult.Limits
	result.Trace = vmResult.Trace
	result.Vars = varsFromVM(vmResult.Vars)
	result.OrgDiff = diffOrg(before, runOrg)
	result.CompletedAt = time.Now().UTC()

	if execErr != nil {
		result.Status = RunStatusRuntimeError
		result.ErrorMessage = execErr.Error()
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: execErr.Error()})
		return result, nil
	}

	if req.Mode == RunModePersist {
		r.store.mu.Lock()
		r.store.org = runOrg.Clone()
		_ = saveDBOrg(r.store.db, r.store.org)
		r.store.mu.Unlock()
	}
	if err := r.cache.Store(cacheKey, result); err != nil {
		return RunResult{}, err
	}
	return result, nil
}

func (r *Runner) loadRuntimeTemplate(workspaceHash string) (*vm.VM, []diagnostic.Diagnostic, error) {
	if r.runtimeTemplate != nil &&
		r.runtimeTemplate.workspaceHash == workspaceHash &&
		r.runtimeTemplate.projectRoot == r.workspace.ProjectRoot &&
		r.runtimeTemplate.version == r.version {
		return r.runtimeTemplate.template, append([]diagnostic.Diagnostic(nil), r.runtimeTemplate.diagnostics...), nil
	}
	index, indexDiagnostics, err := r.loadWorkspaceIndex(r.workspace.ProjectRoot)
	if err != nil {
		return nil, nil, err
	}
	template := vm.New(nil)
	template.SetTraceEnabled(true)
	if err := apextest.RegisterProjectRuntimeForRequest(template, index); err != nil {
		return nil, indexDiagnostics, err
	}
	r.runtimeTemplate = &cachedRuntimeTemplate{
		workspaceHash: workspaceHash,
		projectRoot:   r.workspace.ProjectRoot,
		version:       r.version,
		template:      template,
		diagnostics:   append([]diagnostic.Diagnostic(nil), indexDiagnostics...),
	}
	return template, append([]diagnostic.Diagnostic(nil), indexDiagnostics...), nil
}

func loadOrCreateDBOrg(path string) storage.OrgState {
	store, err := storage.OpenSQLite(path)
	if err != nil {
		return baselineOrg()
	}
	defer store.Close()
	org, err := store.Load()
	if err != nil || len(org.Objects) == 0 {
		org = baselineOrg()
		_ = store.Save(org)
	}
	return org
}

func saveDBOrg(path string, org storage.OrgState) error {
	if path == "" {
		return nil
	}
	store, err := storage.OpenSQLite(path)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Save(org)
}

func loadWorkspaceIndex(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, nil, err
	}
	p.Namespace = ""
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, oaerschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
		return index, index.Diagnostics, nil
	}
	index := typesys.Build(p, s)
	return index, index.Diagnostics, nil
}

func diagnosticsFromIndex(in []diagnostic.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in {
		out = append(out, Diagnostic{Severity: string(d.Severity), Message: d.Message, Line: d.Range.Start.Line, Column: d.Range.Start.Column})
	}
	return out
}

func varsFromVM(vars map[string]vm.Value) []VarResult {
	out := make([]VarResult, 0, len(vars))
	for name, value := range vars {
		out = append(out, VarResult{Name: name, Type: valueType(value), Value: value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func valueType(value vm.Value) string {
	if value.Type != "" {
		return value.Type
	}
	return string(value.Kind)
}

func baselineOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = "65.0"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func runID(t time.Time) string {
	return strings.ReplaceAll(t.Format("20060102T150405.000000000Z07:00"), ":", "")[:24]
}

func millisSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
