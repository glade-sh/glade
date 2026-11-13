package gladecli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/frontendcache"
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
func loadLSPProject(root string) (typesys.Index, func(typesys.Index) sema.Result, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, nil, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
		return index, nil, nil
	}
	cacheAllowed := apextest.ResolveDiskRuntimeCachePolicy(apextest.Options{}).Enabled
	var index typesys.Index
	var artifacts typesys.BuildArtifacts
	if cacheAllowed {
		frontend := frontendcache.LoadOrBuild(p, s, func() (typesys.Index, typesys.BuildArtifacts) {
			return typesys.BuildWithArtifacts(p, s)
		})
		index, artifacts = frontend.Index, frontend.Artifacts
	} else {
		index, artifacts = typesys.BuildWithArtifacts(p, s)
	}
	return index, func(in typesys.Index) sema.Result {
		result, _ := cachedLSPAnalysis(in, &artifacts, cacheAllowed, lspSemanticResults)
		return result
	}, nil
}

// cachedLSPAnalysis returns the diagnostics-and-types semantic result for
// index, from memory or disk when the captured inputs match. Any cache or
// generation problem falls back to a direct analysis and an empty source.
func cachedLSPAnalysis(index typesys.Index, artifacts *typesys.BuildArtifacts, cacheAllowed bool, manager *semanticcache.Manager) (sema.Result, semanticcache.Source) {
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
		analyzed := sema.AnalyzeWithOptions(analysisIndex, options)
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
