package lwcbrowser

import (
	"path/filepath"

	"github.com/glade-sh/glade/internal/aura"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

func PreparePageConfig(p project.Project, cacheRoot string) (PageConfig, compile.Manifest, error) {
	outDir := filepath.Join(cacheRoot, "lwc")
	compiled, err := compile.Compile(p, compile.Options{OutDir: outDir})
	if err != nil {
		return PageConfig{}, compile.Manifest{}, err
	}
	namespace := p.Namespace
	if namespace == "" {
		namespace = "c"
	}
	auraIdx, err := aura.BuildIndex(p)
	if err != nil {
		return PageConfig{}, compile.Manifest{}, err
	}
	manifest := ManifestFromCompile(compiled, "/lightning/modules")
	passthroughs, err := aura.BuildLWCPassthroughIndex(p)
	if err != nil {
		return PageConfig{}, compile.Manifest{}, err
	}
	ApplyAuraLWCPassthroughAliases(manifest, passthroughs, namespace)
	cfg := PageConfig{
		Namespace:          namespace,
		OutApps:            OutAppQualifiedNames(auraIdx.OutApps, namespace),
		OutAppDependencies: OutAppDependencyMap(auraIdx.OutApps, namespace),
		Manifest:           manifest,
	}
	return cfg, compiled, nil
}
