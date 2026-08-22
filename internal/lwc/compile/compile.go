package compile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/project"
)

type Options struct {
	OutDir    string
	Namespace string
	RepoRoot  string
}

type ModuleEntry struct {
	ModuleKey string `json:"moduleKey"`
	Tag       string `json:"tag"`
	File      string `json:"file"`
}

type Manifest struct {
	Modules map[string]ModuleEntry `json:"modules"`
	OutDir  string
}

type compileConfig struct {
	ProjectRoot           string                       `json:"projectRoot"`
	OutDir                string                       `json:"outDir"`
	Namespace             string                       `json:"namespace"`
	LWCFiles              []string                     `json:"lwcFiles"`
	LWCHTMLFiles          []string                     `json:"lwcHtmlFiles"`
	LWCMetaFiles          []string                     `json:"lwcMetaFiles"`
	LWCAPIVersions        map[string]int               `json:"lwcApiVersions"`
	LWCModuleAvailability map[string]apexversion.Range `json:"lwcModuleAvailability"`
}

type compileResult struct {
	Modules      map[string]ModuleEntry `json:"modules"`
	ManifestPath string                 `json:"manifestPath"`
}

type compileRoots struct {
	ScriptRoot     string
	DependencyRoot string
}

func Compile(p project.Project, opts Options) (Manifest, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return Manifest{}, fmt.Errorf("compile: OutDir is required")
	}
	roots := compileRoots{
		ScriptRoot:     strings.TrimSpace(opts.RepoRoot),
		DependencyRoot: strings.TrimSpace(opts.RepoRoot),
	}
	if roots.ScriptRoot == "" {
		var err error
		roots, err = compileToolchainRoots()
		if err != nil {
			return Manifest{}, err
		}
	}
	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(p.Namespace)
	}
	if namespace == "" {
		namespace = "c"
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return Manifest{}, err
	}
	projectRoot, err := filepath.Abs(p.Root)
	if err != nil {
		return Manifest{}, err
	}
	cfg, err := buildCompileConfig(p, projectRoot, outDir, namespace)
	if err != nil {
		return Manifest{}, err
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return Manifest{}, err
	}
	script := filepath.Join(roots.ScriptRoot, "third_party", "lwc", "compile.mjs")
	cmd := exec.Command("node", script)
	toolchainDir := filepath.Join(roots.DependencyRoot, "third_party", "lwc")
	cmd.Dir = toolchainDir
	cmd.Env = append(os.Environ(), "GLADE_LWC_TOOLCHAIN_DIR="+toolchainDir)
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Manifest{}, fmt.Errorf("lwc compile: %w\n%s", err, string(out))
	}
	var result compileResult
	if err := json.Unmarshal(out, &result); err != nil {
		return Manifest{}, fmt.Errorf("decode compile result: %w\n%s", err, string(out))
	}
	if result.Modules == nil {
		result.Modules = map[string]ModuleEntry{}
	}
	return Manifest{Modules: result.Modules, OutDir: outDir}, nil
}

func buildCompileConfig(p project.Project, projectRoot, outDir, namespace string) (compileConfig, error) {
	versions := make(map[string]int, len(p.LWCMetaFiles))
	roots := make([]string, 0, len(p.LWCMetaFiles))
	for _, metaPath := range p.LWCMetaFiles {
		root := filepath.Clean(filepath.Dir(metaPath))
		relative, err := filepath.Rel(projectRoot, root)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return compileConfig{}, fmt.Errorf("LWC metadata escapes project root: %s", metaPath)
		}
		key := filepath.ToSlash(relative)
		if _, exists := versions[key]; exists {
			return compileConfig{}, fmt.Errorf("multiple LWC metadata files for bundle %s", key)
		}
		meta, err := lwc.ParseComponentMeta(metaPath)
		if err != nil {
			return compileConfig{}, err
		}
		if strings.TrimSpace(meta.APIVersion) == "" {
			return compileConfig{}, fmt.Errorf("missing component API version: %s", filepath.ToSlash(metaPath))
		}
		resolved, err := apexversion.ResolveSource(meta.APIVersion)
		if err != nil {
			return compileConfig{}, fmt.Errorf("%s: %w", filepath.ToSlash(metaPath), err)
		}
		major, _ := apexversion.Major(resolved)
		versions[key] = major
		roots = append(roots, root)
	}
	for _, sourcePath := range append(append([]string{}, p.LWCFiles...), p.LWCHTMLFiles...) {
		matches := 0
		for _, root := range roots {
			relative, err := filepath.Rel(root, sourcePath)
			if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				matches++
			}
		}
		if matches != 1 {
			return compileConfig{}, fmt.Errorf("LWC source %s belongs to %d metadata bundles", filepath.ToSlash(sourcePath), matches)
		}
	}
	return compileConfig{
		ProjectRoot:           projectRoot,
		OutDir:                outDir,
		Namespace:             namespace,
		LWCFiles:              relativize(projectRoot, p.LWCFiles),
		LWCHTMLFiles:          relativize(projectRoot, p.LWCHTMLFiles),
		LWCMetaFiles:          relativize(projectRoot, p.LWCMetaFiles),
		LWCAPIVersions:        versions,
		LWCModuleAvailability: generatedLWCModuleAvailability,
	}, nil
}

// FindRepoRoot returns the glade source checkout (for testdata and development files).
func FindRepoRoot() (string, error) {
	return gladehome.RepoRoot()
}

func compileToolchainRoots() (compileRoots, error) {
	dependencyRoot, err := gladehome.EnsureRoot()
	if err != nil {
		return compileRoots{}, err
	}
	roots := compileRoots{
		ScriptRoot:     dependencyRoot,
		DependencyRoot: dependencyRoot,
	}
	if sourceRoot, err := gladehome.SourceRoot(); err == nil {
		if _, err := os.Stat(filepath.Join(sourceRoot, "third_party", "lwc", "compile.mjs")); err == nil {
			roots.ScriptRoot = sourceRoot
		}
	}
	return roots, nil
}

func relativize(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}
