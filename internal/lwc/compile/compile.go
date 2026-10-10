package compile

import (
	"encoding/json"
	"errors"
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
	// LightningModules is the host module registry. Nil leaves external module
	// resolution to callers that do not supply a browser host.
	LightningModules map[string]string
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

// Diagnostic is the compiler's original code and message, before stack formatting.
type Diagnostic struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// BundleResult contains one isolated bundle compilation. Err does not prevent
// other bundles from compiling. Diagnostics retain the compiler's exact text.
type BundleResult struct {
	Manifest    Manifest
	Err         error
	Diagnostics []Diagnostic
	// DeploymentDiagnostics include source positions and deployment envelopes.
	// Diagnostics above retain the underlying compiler's original messages.
	DeploymentDiagnostics []string
	ReportedDiagnostics   []string // Deployment messages, including original source positions.
}

type compileConfig struct {
	Batch                   bool                         `json:"batch,omitempty"`
	BundleOutDirs           map[string]string            `json:"bundleOutDirs,omitempty"`
	ProjectRoot             string                       `json:"projectRoot"`
	OutDir                  string                       `json:"outDir"`
	Namespace               string                       `json:"namespace"`
	LWCFiles                []string                     `json:"lwcFiles"`
	LWCHTMLFiles            []string                     `json:"lwcHtmlFiles"`
	LWCMetaFiles            []string                     `json:"lwcMetaFiles"`
	LWCAPIVersions          map[string]int               `json:"lwcApiVersions"`
	LWCModuleAvailability   map[string]apexversion.Range `json:"lwcModuleAvailability"`
	LWCConfiguredProperties map[string][]string          `json:"lwcConfiguredProperties"`
	LWCCapabilities         map[string][]string          `json:"lwcCapabilities"`
	LWCCapabilityErrors     map[string]string            `json:"lwcCapabilityErrors"`
	LightningModules        map[string]string            `json:"lightningModules"`
	SalesforceImports       *salesforceImportConfig      `json:"salesforceImports,omitempty"`
}

type bundleCompileResult struct {
	Modules               map[string]ModuleEntry `json:"modules"`
	Error                 string                 `json:"error"`
	Diagnostics           []Diagnostic           `json:"diagnostics"`
	DeploymentDiagnostics []string               `json:"deploymentDiagnostics"`
	ReportedDiagnostics   []string               `json:"reportedDiagnostics"`
}

type compileResult struct {
	Bundles      map[string]bundleCompileResult `json:"bundles"`
	Modules      map[string]ModuleEntry         `json:"modules"`
	ManifestPath string                         `json:"manifestPath"`
}

type compileRoots struct {
	ScriptRoot     string
	DependencyRoot string
}

const (
	minimumLWCSourceAPIVersion = 59
	maximumLWCSourceAPIVersion = 67
)

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
	cfg.LightningModules = opts.LightningModules
	result, err := runCompiler(p, cfg, roots)
	if err != nil {
		return Manifest{}, err
	}
	if result.Modules == nil {
		result.Modules = map[string]ModuleEntry{}
	}
	return Manifest{Modules: result.Modules, OutDir: outDir}, nil
}

// CompileBatch compiles independent bundles from one project in one Node run.
// Results are keyed by the slash-separated bundle path relative to p.Root.
// Bundle names and namespace stay unchanged, even when different paths have
// the same name. Each bundle gets its own output directory under opts.OutDir.
// Only setup/process failures are returned as the outer error.
func CompileBatch(p project.Project, opts Options) (map[string]BundleResult, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return nil, fmt.Errorf("compile: OutDir is required")
	}
	roots := compileRoots{ScriptRoot: strings.TrimSpace(opts.RepoRoot), DependencyRoot: strings.TrimSpace(opts.RepoRoot)}
	if roots.ScriptRoot == "" {
		var err error
		roots, err = compileToolchainRoots()
		if err != nil {
			return nil, err
		}
	}
	projectRoot, err := filepath.Abs(p.Root)
	if err != nil {
		return nil, err
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(p.Namespace)
	}
	if namespace == "" {
		namespace = "c"
	}
	cfg := compileConfig{Batch: true, ProjectRoot: projectRoot, OutDir: outDir, Namespace: namespace,
		BundleOutDirs: map[string]string{}, LWCAPIVersions: map[string]int{},
		LWCCapabilities: map[string][]string{}, LWCCapabilityErrors: map[string]string{},
		LWCConfiguredProperties: map[string][]string{}, LWCModuleAvailability: generatedLWCModuleAvailability, LightningModules: opts.LightningModules}
	results := map[string]BundleResult{}
	// Group metadata first so duplicate declarations still receive the same
	// validation error as Compile, rather than overwriting a previous result.
	bundles := map[string][]string{}
	var order []string
	for _, meta := range p.LWCMetaFiles {
		dir := filepath.Clean(filepath.Dir(meta))
		key, err := filepath.Rel(projectRoot, dir)
		if err != nil || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("LWC metadata escapes project root: %s", meta)
		}
		key = filepath.ToSlash(key)
		if _, exists := bundles[key]; !exists {
			order = append(order, key)
		}
		bundles[key] = append(bundles[key], meta)
	}
	// Do not silently discard sources without metadata or ambiguously nested
	// bundles. These project-level errors have the same text as Compile.
	for _, source := range append(append([]string{}, p.LWCFiles...), p.LWCHTMLFiles...) {
		matches := 0
		for _, key := range order {
			if len(filesInBundle(filepath.Join(projectRoot, filepath.FromSlash(key)), []string{source})) != 0 {
				matches++
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("LWC source %s belongs to %d metadata bundles", filepath.ToSlash(source), matches)
		}
	}
	for _, key := range order {
		dir := filepath.Join(projectRoot, filepath.FromSlash(key))
		bundleProject := p
		bundleProject.LWCMetaFiles = bundles[key]
		bundleProject.LWCFiles = filesInBundle(dir, p.LWCFiles)
		bundleProject.LWCHTMLFiles = filesInBundle(dir, p.LWCHTMLFiles)
		bundleOut := filepath.Join(outDir, filepath.FromSlash(key))
		result := BundleResult{Manifest: Manifest{Modules: map[string]ModuleEntry{}, OutDir: bundleOut}, Diagnostics: []Diagnostic{}, ReportedDiagnostics: []string{}}
		bundleConfig, err := buildCompileConfig(bundleProject, projectRoot, bundleOut, namespace)
		if err != nil {
			result.Err = err
			var metadataError *lwc.MetadataValidationError
			if errors.As(err, &metadataError) {
				for _, message := range metadataError.Messages {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Message: message})
					result.ReportedDiagnostics = append(result.ReportedDiagnostics, message)
					result.DeploymentDiagnostics = append(result.DeploymentDiagnostics, message)
				}
			} else {
				diagnosticErr := err
				if cause := errors.Unwrap(err); cause != nil {
					diagnosticErr = cause
				}
				result.ReportedDiagnostics = []string{diagnosticErr.Error()}
			}
		} else {
			cfg.LWCMetaFiles = append(cfg.LWCMetaFiles, bundleConfig.LWCMetaFiles...)
			cfg.LWCFiles = append(cfg.LWCFiles, bundleConfig.LWCFiles...)
			cfg.LWCHTMLFiles = append(cfg.LWCHTMLFiles, bundleConfig.LWCHTMLFiles...)
			cfg.LWCAPIVersions[key] = bundleConfig.LWCAPIVersions[key]
			cfg.LWCConfiguredProperties[key] = bundleConfig.LWCConfiguredProperties[key]
			cfg.LWCCapabilities[key] = bundleConfig.LWCCapabilities[key]
			cfg.LWCCapabilityErrors[key] = bundleConfig.LWCCapabilityErrors[key]
			cfg.BundleOutDirs[key] = bundleOut
		}
		results[key] = result
	}
	if len(cfg.LWCMetaFiles) == 0 {
		return results, nil
	}
	compiled, err := runCompiler(p, cfg, roots)
	if err != nil {
		return nil, err
	}
	for key := range cfg.BundleOutDirs {
		bundle, ok := compiled.Bundles[key]
		if !ok {
			return nil, fmt.Errorf("decode compile result: missing bundle %s", key)
		}
		result := results[key]
		if bundle.Modules != nil {
			result.Manifest.Modules = bundle.Modules
		}
		if bundle.Diagnostics != nil {
			result.Diagnostics = bundle.Diagnostics
		}
		result.DeploymentDiagnostics = bundle.DeploymentDiagnostics
		if bundle.ReportedDiagnostics != nil {
			result.ReportedDiagnostics = bundle.ReportedDiagnostics
		}
		if bundle.Error != "" {
			result.Err = fmt.Errorf("lwc compile: exit status 1\n%s\n", bundle.Error)
		}
		results[key] = result
	}
	return results, nil
}

func filesInBundle(root string, paths []string) []string {
	var files []string
	for _, file := range paths {
		rel, err := filepath.Rel(root, file)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			files = append(files, file)
		}
	}
	return files
}

func runCompiler(p project.Project, cfg compileConfig, roots compileRoots) (compileResult, error) {
	// Both invocation shapes hydrate the same decoded import references before
	// entering the shared bundle compiler and its unchanged diagnostic path.
	references, err := collectSalesforceImportReferences(cfg, roots)
	if err != nil {
		return compileResult{}, err
	}
	cfg.SalesforceImports, err = loadSalesforceImports(p, references)
	if err != nil {
		return compileResult{}, err
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return compileResult{}, err
	}
	cmd := lwcCompilerCommand(roots)
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return compileResult{}, fmt.Errorf("lwc compile: %w\n%s", err, string(out))
	}
	var result compileResult
	if err := json.Unmarshal(out, &result); err != nil {
		return compileResult{}, fmt.Errorf("decode compile result: %w\n%s", err, string(out))
	}
	return result, nil
}

func lwcCompilerCommand(roots compileRoots) *exec.Cmd {
	script := filepath.Join(roots.ScriptRoot, "third_party", "lwc", "compile.mjs")
	cmd := exec.Command("node", script)
	toolchainDir := filepath.Join(roots.DependencyRoot, "third_party", "lwc")
	cmd.Dir = toolchainDir
	cmd.Env = append(os.Environ(), "GLADE_LWC_TOOLCHAIN_DIR="+toolchainDir)
	return cmd
}

func buildCompileConfig(p project.Project, projectRoot, outDir, namespace string) (compileConfig, error) {
	versions := make(map[string]int, len(p.LWCMetaFiles))
	properties := make(map[string][]string, len(p.LWCMetaFiles))
	capabilities := make(map[string][]string, len(p.LWCMetaFiles))
	capabilityErrors := make(map[string]string, len(p.LWCMetaFiles))
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
			return compileConfig{}, fmt.Errorf("%s: %w", filepath.ToSlash(metaPath), err)
		}
		if err := meta.ValidateCompilerConfiguration(); err != nil {
			return compileConfig{}, fmt.Errorf("%s: %w", filepath.ToSlash(metaPath), err)
		}
		capabilities[key] = meta.Capabilities
		if err := meta.ValidateCapabilities(); err != nil {
			capabilityErrors[key] = err.Error()
		}
		version := meta.APIVersion
		if version == "" {
			// Unversioned bundles use the compiler default, independently of
			// the project's deployment API. Explicit bundle versions stay exact.
			version = fmt.Sprintf("%d.0", maximumLWCSourceAPIVersion)
		}
		major, err := resolveLWCSourceAPIVersion(version)
		if err != nil {
			return compileConfig{}, fmt.Errorf("%s: %w", filepath.ToSlash(metaPath), err)
		}
		versions[key] = major
		for _, cfg := range meta.TargetConfigs {
			for _, prop := range cfg.Properties {
				properties[key] = append(properties[key], prop.Name)
			}
		}
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
		ProjectRoot:             projectRoot,
		OutDir:                  outDir,
		Namespace:               namespace,
		LWCFiles:                relativize(projectRoot, p.LWCFiles),
		LWCHTMLFiles:            relativize(projectRoot, p.LWCHTMLFiles),
		LWCMetaFiles:            relativize(projectRoot, p.LWCMetaFiles),
		LWCAPIVersions:          versions,
		LWCModuleAvailability:   generatedLWCModuleAvailability,
		LWCConfiguredProperties: properties,
		LWCCapabilities:         capabilities,
		LWCCapabilityErrors:     capabilityErrors,
	}, nil
}

func resolveLWCSourceAPIVersion(raw string) (int, error) {
	resolved, err := apexversion.PreserveSource(raw)
	if err != nil {
		return 0, fmt.Errorf("unsupported LWC source API version %q; supported versions: %d.0 through %d.0", raw, minimumLWCSourceAPIVersion, maximumLWCSourceAPIVersion)
	}
	major, ok := apexversion.Major(resolved)
	if !ok || major < minimumLWCSourceAPIVersion || major > maximumLWCSourceAPIVersion {
		return 0, fmt.Errorf("unsupported LWC source API version %q; supported versions: %d.0 through %d.0", raw, minimumLWCSourceAPIVersion, maximumLWCSourceAPIVersion)
	}
	return major, nil
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
