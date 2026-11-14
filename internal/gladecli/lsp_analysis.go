package gladecli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/frontendcache"
	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

var lspSemanticResults = semanticcache.NewManager(semanticcache.Limits{MaxEntries: 2, MaxBytes: 512 << 20})

// loadLSPProject builds the project index for the language server and returns
// the semantic pass to run on it. The pass reads and writes the same disk
// semantic cache as check, so a project analyzed once opens without a rerun.
// A nil analyzer means the plain sema.Analyze pass.
type lspProjectLoader struct {
	root         string
	project      project.Project
	schema       gladeschema.Schema
	schemaErr    error
	index        typesys.Index
	artifacts    typesys.BuildArtifacts
	cacheAllowed bool
	state        *sema.IncrementalState
	mu           sync.Mutex
}

func loadLSPProject(root string) (typesys.Index, func(typesys.Index) sema.Result, error) {
	loader, err := newLSPProjectLoader(root)
	if err != nil {
		return typesys.Index{}, nil, err
	}
	return loader.index, loader.analyzer(loader.artifacts), nil
}

func newLSPProjectLoader(root string) (*lspProjectLoader, error) {
	l := &lspProjectLoader{root: root, cacheAllowed: apextest.ResolveDiskRuntimeCachePolicy(apextest.Options{}).Enabled, state: sema.NewIncrementalState()}
	if err := l.load(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *lspProjectLoader) load() error {
	p, err := project.Load(l.root)
	if err != nil {
		return err
	}
	schema, err := gladeschema.LoadProject(p)
	l.project, l.schema, l.schemaErr = p, schema, err
	if err != nil {
		l.index = typesys.Build(p, gladeschema.Schema{})
		l.index.Diagnostics = append(l.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESCHEMA001", Message: fmt.Sprintf("metadata schema load failed: %v", err)})
		l.artifacts = typesys.BuildArtifacts{}
		return nil
	}
	if l.cacheAllowed {
		frontend := frontendcache.LoadOrBuild(p, schema, func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, schema) })
		l.index, l.artifacts = frontend.Index, frontend.Artifacts
	} else {
		l.index, l.artifacts = typesys.BuildWithArtifacts(p, schema)
	}
	return nil
}

func (l *lspProjectLoader) analyzer(artifacts typesys.BuildArtifacts) func(typesys.Index) sema.Result {
	return func(index typesys.Index) sema.Result {
		result, _ := cachedLSPAnalysisWithCompute(index, &artifacts, l.cacheAllowed, lspSemanticResults, func(i typesys.Index, o sema.AnalyzeOptions) sema.Result {
			result, _ := l.state.Analyze(i, o)
			return result
		})
		return result
	}
}

func (l *lspProjectLoader) refreshIndex(ctx context.Context, paths []string) (typesys.Index, func(typesys.Index) sema.Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return typesys.Index{}, nil, err
	}
	if len(paths) == 1 && (strings.EqualFold(filepath.Ext(paths[0]), ".cls") || strings.EqualFold(filepath.Ext(paths[0]), ".trigger")) {
		p, err := project.Load(l.root)
		if err == nil {
			next, exact, updateErr := typesys.TryUpdateApexFilesCheckedWithLoadedProject(l.index, paths, nil, p)
			if updateErr == nil && exact {
				artifacts, artifactErr := typesys.RefreshBuildArtifacts(next, &l.artifacts)
				if artifactErr == nil {
					l.index, l.artifacts = next, artifacts
					l.project = p
					return next, l.analyzer(artifacts), nil
				}
			}
		}
		if _, err := os.Stat(paths[0]); os.IsNotExist(err) {
			if err := l.load(); err != nil {
				return typesys.Index{}, nil, err
			}
			return l.index, l.analyzer(l.artifacts), nil
		}
		data, parseErr := os.ReadFile(paths[0])
		if parseErr != nil {
			return typesys.Index{}, nil, parseErr
		}
		parser := apexast.NewParser()
		parsed := parser.ParseSource(paths[0], string(data))
		parser.Close()
		if len(parsed.Diagnostics) > 0 {
			return typesys.Index{}, nil, &lsp.ParseDiagnosticsError{Path: paths[0], Digest: sha256.Sum256(data), Diagnostics: parsed.Diagnostics}
		}
	}
	if err := l.load(); err != nil {
		return typesys.Index{}, nil, err
	}
	return l.index, l.analyzer(l.artifacts), nil
}

func (l *lspProjectLoader) base() lsp.Base {
	artifacts := l.artifacts
	return lsp.Base{Index: l.index, Project: l.project, Artifacts: &artifacts, Analyze: l.analyzer(artifacts), AnalyzeOverlay: l.analyzeOverlay, RebuildOverlay: l.rebuildOverlay}
}

func (l *lspProjectLoader) refresh(ctx context.Context, request lsp.RefreshRequest) (lsp.Base, error) {
	paths := request.Changed
	if len(request.Created) != 0 || len(request.Deleted) != 0 {
		paths = nil
	}
	_, _, err := l.refreshIndex(ctx, paths)
	if err != nil {
		return lsp.Base{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.base(), nil
}

func (l *lspProjectLoader) analyzeOverlay(index typesys.Index, artifacts *typesys.BuildArtifacts) sema.Result {
	result, _ := l.state.Analyze(apextest.SemanticAnalysisIndex(index), sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: artifacts})
	return result
}

func (l *lspProjectLoader) rebuildOverlay(ctx context.Context, overlay *typesys.SourceOverlay) (typesys.Index, typesys.BuildArtifacts, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return typesys.Index{}, typesys.BuildArtifacts{}, err
	}
	if l.schemaErr != nil {
		return typesys.Index{}, typesys.BuildArtifacts{}, l.schemaErr
	}
	idx, artifacts := typesys.BuildWithArtifactsOverlay(l.project, l.schema, overlay)
	return idx, artifacts, nil
}

// cachedLSPAnalysis returns the diagnostics-and-types semantic result for
// index, from memory or disk when the captured inputs match. Any cache or
// generation problem falls back to a direct analysis and an empty source.
func cachedLSPAnalysis(index typesys.Index, artifacts *typesys.BuildArtifacts, cacheAllowed bool, manager *semanticcache.Manager) (sema.Result, semanticcache.Source) {
	return cachedLSPAnalysisWithCompute(index, artifacts, cacheAllowed, manager, sema.AnalyzeWithOptions)
}

func cachedLSPAnalysisWithCompute(index typesys.Index, artifacts *typesys.BuildArtifacts, cacheAllowed bool, manager *semanticcache.Manager, compute func(typesys.Index, sema.AnalyzeOptions) sema.Result) (sema.Result, semanticcache.Source) {
	options := sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: artifacts}
	if typesys.ValidateBuildGeneration(index, artifacts) != nil {
		return sema.Analyze(index), ""
	}
	analysisIndex := apextest.SemanticAnalysisIndex(index)
	identity, err := semanticcache.IdentityForBuild(analysisIndex, artifacts, options)
	if err != nil {
		return sema.Analyze(index), ""
	}
	cachePath := filepath.Join(".glade", "semantic", "result-"+identity.OptionsFingerprint+".json")
	result, access, err := manager.GetOrCompute(context.Background(), semanticcache.Request{
		Identity:     identity,
		ProjectRoot:  index.Project.Root,
		RelativePath: cachePath,
		NoDisk:       !cacheAllowed,
		BypassMemory: !cacheAllowed,
	}, func() (sema.Result, error) {
		analyzed := compute(analysisIndex, options)
		if err := typesys.ValidateBuildGeneration(index, artifacts); err != nil {
			return sema.Result{}, err
		}
		return analyzed, nil
	})
	if err != nil {
		return sema.Analyze(index), ""
	}
	if typesys.ValidateBuildGeneration(index, artifacts) != nil {
		manager.Evict(identity)
		return sema.Analyze(index), ""
	}
	return result, access.Source
}

func (l *lspProjectLoader) valid(index typesys.Index) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return typesys.ValidateBuildGeneration(index, &l.artifacts) == nil
}
