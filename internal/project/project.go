package project

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/namespaceremap"
)

// EffectiveSourceAPIVersion returns an Apex source file's companion metadata
// apiVersion when present, otherwise the project's configured API version.
func EffectiveSourceAPIVersion(path, fallback string) string {
	data, err := os.ReadFile(path + "-meta.xml") // #nosec G304 -- path is an indexed Apex source path; this reads its fixed companion metadata file.
	if err != nil {
		return fallback
	}
	return EffectiveSourceAPIVersionFromMetadata(data, fallback)
}

// EffectiveSourceAPIVersionFromMetadata derives the effective API version
// from already-captured companion metadata bytes. Callers building an
// immutable source generation use this to avoid rereading a sidecar after its
// identity has been recorded.
func EffectiveSourceAPIVersionFromMetadata(data []byte, fallback string) string {
	var metadata struct {
		APIVersion string `xml:"apiVersion"`
	}
	if err := xml.Unmarshal(data, &metadata); err != nil || strings.TrimSpace(metadata.APIVersion) == "" {
		return fallback
	}
	return strings.TrimSpace(metadata.APIVersion)
}

type Project struct {
	Root                       string                     `json:"root"`
	Namespace                  string                     `json:"namespace,omitempty"`
	SourceAPIVersion           string                     `json:"sourceApiVersion,omitempty"`
	PackageDirectories         []PackageDirectory         `json:"packageDirectories"`
	ApexFiles                  []string                   `json:"apexFiles"`
	ObjectFiles                []string                   `json:"objectFiles"`
	FieldFiles                 []string                   `json:"fieldFiles"`
	FieldSetFiles              []string                   `json:"fieldSetFiles"`
	RecordTypeFiles            []string                   `json:"recordTypeFiles"`
	ValidationRuleFiles        []string                   `json:"validationRuleFiles"`
	LabelFiles                 []string                   `json:"labelFiles"`
	TranslationFiles           []string                   `json:"translationFiles,omitempty"`
	StaticResourceFiles        []string                   `json:"staticResourceFiles"`
	StaticResourceMetas        []string                   `json:"staticResourceMetas"`
	DataCategoryGroupFiles     []string                   `json:"dataCategoryGroupFiles,omitempty"`
	DataWeaveFiles             []string                   `json:"dataWeaveFiles,omitempty"`
	DataWeaveMetas             []string                   `json:"dataWeaveMetas,omitempty"`
	ContentAssetFiles          []string                   `json:"contentAssetFiles,omitempty"`
	ContentAssetMetas          []string                   `json:"contentAssetMetas,omitempty"`
	EmailTemplateFiles         []string                   `json:"emailTemplateFiles,omitempty"`
	FolderFiles                []string                   `json:"folderFiles,omitempty"`
	NamedCredentialFiles       []string                   `json:"namedCredentialFiles"`
	RemoteSiteFiles            []string                   `json:"remoteSiteFiles"`
	CustomMetadataFiles        []string                   `json:"customMetadataFiles"`
	WorkflowFiles              []string                   `json:"workflowFiles"`
	FlowFiles                  []string                   `json:"flowFiles"`
	ProfileFiles               []string                   `json:"profileFiles"`
	PermissionSetFiles         []string                   `json:"permissionSetFiles"`
	PermissionSetGroupFiles    []string                   `json:"permissionSetGroupFiles,omitempty"`
	PermissionAssignmentFiles  []string                   `json:"permissionAssignmentFiles"`
	ListViewFiles              []string                   `json:"listViewFiles"`
	LayoutFiles                []string                   `json:"layoutFiles"`
	CompactLayoutFiles         []string                   `json:"compactLayoutFiles"`
	TabFiles                   []string                   `json:"tabFiles"`
	WebLinkFiles               []string                   `json:"webLinkFiles"`
	QuickActionFiles           []string                   `json:"quickActionFiles"`
	GlobalValueSetFiles        []string                   `json:"globalValueSetFiles"`
	StandardValueSetFiles      []string                   `json:"standardValueSetFiles"`
	FlexiPageFiles             []string                   `json:"flexiPageFiles"`
	ApplicationFiles           []string                   `json:"applicationFiles"`
	VisualforcePageFiles       []string                   `json:"visualforcePageFiles"`
	VisualforceComponentFiles  []string                   `json:"visualforceComponentFiles"`
	AuraFiles                  []string                   `json:"auraFiles"`
	LWCFiles                   []string                   `json:"lwcFiles"`
	LWCHTMLFiles               []string                   `json:"lwcHtmlFiles"`
	LWCCSSFiles                []string                   `json:"lwcCssFiles"`
	LWCMetaFiles               []string                   `json:"lwcMetaFiles"`
	NamespaceRemaps            []namespaceremap.Rule      `json:"namespaceRemaps,omitempty"`
	ManagedPackageDependencies []ManagedPackageDependency `json:"managedPackageDependencies,omitempty"`
	PackageShims               []PackageShim              `json:"packageShims,omitempty"`
	DependencyDiagnostics      []DependencyDiagnostic     `json:"dependencyDiagnostics,omitempty"`
}

type ManagedPackageDependency struct {
	Namespace    string   `json:"namespace"`
	SourceRoot   string   `json:"sourceRoot,omitempty"`
	ArtifactPath string   `json:"artifactPath,omitempty"`
	Version      string   `json:"version,omitempty"`
	Project      *Project `json:"project,omitempty"`
	Status       string   `json:"status"`
}

type PackageShim struct {
	Namespace  string   `json:"namespace"`
	SourceRoot string   `json:"sourceRoot"`
	Project    *Project `json:"project,omitempty"`
	Status     string   `json:"status"`
}

type DependencyDiagnostic struct {
	Namespace  string `json:"namespace,omitempty"`
	SourceRoot string `json:"sourceRoot,omitempty"`
	Version    string `json:"version,omitempty"`
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func NormalizeApexNamespaceTokens(source, namespace string) string {
	namespace = strings.TrimSpace(namespace)
	apiPrefix := ""
	dotPrefix := ""
	if namespace != "" {
		apiPrefix = namespace + "__"
		dotPrefix = namespace + "."
	}
	source = strings.ReplaceAll(source, "%%%NAMESPACE_DOT%%%", dotPrefix)
	source = strings.ReplaceAll(source, "%%%NAMESPACE%%%", apiPrefix)
	return source
}

type PackageDirectory struct {
	Path         string              `json:"path"`
	Default      bool                `json:"default,omitempty"`
	Package      string              `json:"package,omitempty"`
	Dependencies []PackageDependency `json:"dependencies,omitempty"`
}

type PackageDependency struct {
	Package       string `json:"package"`
	VersionNumber string `json:"versionNumber,omitempty"`
}

type scratchOrgDefinition struct {
	Features []string       `json:"features"`
	Settings map[string]any `json:"settings"`
}

func OrgShapeFeatures(root string) []string {
	features := make([]string, 0)
	if cfg, _, err := config.LoadNearest(root); err == nil {
		features = append(features, cfg.Org.Features...)
	}
	for _, name := range []string{
		"project-scratch-def.json",
		"hc-project-scratch-def.json",
	} {
		path := filepath.Join(root, "config", name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var def scratchOrgDefinition
		if err := json.Unmarshal(data, &def); err == nil {
			features = append(features, def.Features...)
			features = append(features, scratchSettingsFeatures(def.Settings)...)
		}
	}
	for _, path := range cumulusCIOrgConfigFiles(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var def scratchOrgDefinition
		if err := json.Unmarshal(data, &def); err == nil {
			features = append(features, def.Features...)
			features = append(features, scratchSettingsFeatures(def.Settings)...)
		}
	}
	return dedupeStrings(features)
}

func cumulusCIOrgConfigFiles(root string) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, name := range []string{"cumulusci.yml", "cumulusci.template.yml"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "config_file:") {
				continue
			}
			path := strings.TrimSpace(strings.TrimPrefix(line, "config_file:"))
			path = strings.Trim(path, `"'`)
			if path == "" || filepath.IsAbs(path) || !strings.HasSuffix(strings.ToLower(path), ".json") {
				continue
			}
			full := filepath.Clean(filepath.Join(root, path))
			if !strings.HasPrefix(full, filepath.Clean(root)+string(os.PathSeparator)) {
				continue
			}
			if !seen[full] {
				seen[full] = true
				paths = append(paths, full)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func scratchSettingsFeatures(settings map[string]any) []string {
	var features []string
	if nestedBool(settings, "communitiesSettings", "enableNetworksEnabled") || nestedBool(settings, "communitiesSettings", "enableOotbProfExtUserOpsEnable") {
		features = append(features, "Communities")
	}
	if nestedBool(settings, "chatterSettings", "enableChatter") {
		features = append(features, "Chatter")
	}
	if nestedBool(settings, "lightningExperienceSettings", "enableS1DesktopEnabled") {
		features = append(features, "LightningExperience")
	}
	if _, ok := settings["userManagementSettings"]; ok {
		features = append(features, "EnableSetPasswordInApi")
	}
	return features
}

func nestedBool(root map[string]any, path ...string) bool {
	var current any = root
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = next[key]
		if !ok {
			return false
		}
	}
	value, ok := current.(bool)
	return ok && value
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeVisualforcePageFiles(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	markup := make(map[string]string, len(paths))
	metadata := make(map[string]string)
	for _, path := range paths {
		key := visualforcePageKey(path)
		if key == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(path), ".page-meta.xml") {
			metadata[key] = path
			continue
		}
		markup[key] = path
	}
	out := make([]string, 0, len(markup)+len(metadata))
	for _, path := range markup {
		out = append(out, path)
	}
	for key, path := range metadata {
		if _, ok := markup[key]; !ok {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

func visualforcePageKey(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(base, ".page-meta.xml"):
		return strings.TrimSuffix(base, ".page-meta.xml")
	case strings.HasSuffix(base, ".page"):
		return strings.TrimSuffix(base, ".page")
	default:
		return ""
	}
}

func (p Project) PackagePathForFile(file string) string {
	absFile, err := filepath.Abs(file)
	if err != nil {
		absFile = file
	}
	absFile = filepath.Clean(absFile)

	bestRoot := ""
	bestPath := ""
	for _, pkg := range p.PackageDirectories {
		if pkg.Path == "" {
			continue
		}
		root := filepath.Clean(filepath.Join(p.Root, filepath.FromSlash(pkg.Path)))
		if absFile != root && !strings.HasPrefix(absFile, root+string(filepath.Separator)) {
			continue
		}
		if len(root) > len(bestRoot) {
			bestRoot = root
			bestPath = filepath.ToSlash(filepath.Clean(pkg.Path))
		}
	}
	return bestPath
}

type sfdxProject struct {
	PackageDirectories []PackageDirectory `json:"packageDirectories"`
	Namespace          string             `json:"namespace"`
	SourceAPIVersion   string             `json:"sourceApiVersion"`
	HasManifest        bool               `json:"-"`
}

func Load(root string) (Project, error) {
	return load(root, nil, false)
}

func load(root string, stack map[string]bool, dependency bool) (Project, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Project{}, err
	}
	absRoot = filepath.Clean(absRoot)
	if stack == nil {
		stack = make(map[string]bool)
	}
	if stack[absRoot] {
		return Project{}, nil
	}
	stack[absRoot] = true
	defer delete(stack, absRoot)

	cfg, err := loadSFDXProject(absRoot)
	if err != nil {
		return Project{}, err
	}
	cfg.SourceAPIVersion, err = apexversion.ResolveSource(cfg.SourceAPIVersion)
	if err != nil {
		return Project{}, err
	}
	cfgPath := filepath.Join(absRoot, "glade.yml")
	gladeCfg, cfgErr := config.LoadFile(cfgPath)
	if errors.Is(cfgErr, os.ErrNotExist) {
		cfgErr = config.ErrNotFound
	}
	if cfgErr == nil {
		cfgDir := filepath.Dir(cfgPath)
		if gladeCfg.Project.Root != "" {
			configuredRoot := gladeCfg.Project.Root
			if !filepath.IsAbs(configuredRoot) {
				configuredRoot = filepath.Join(cfgDir, filepath.FromSlash(configuredRoot))
			}
			if cleaned := filepath.Clean(configuredRoot); cleaned != absRoot {
				return load(cleaned, stack, dependency)
			}
		}
		if len(gladeCfg.Project.PackageDirs) > 0 {
			cfg.PackageDirectories = packageDirectoriesFromConfig(gladeCfg.Project.PackageDirs)
			cfg.HasManifest = true
		}
		if gladeCfg.Project.DefaultNamespace != "" {
			cfg.Namespace = gladeCfg.Project.DefaultNamespace
		}
	}
	if len(cfg.PackageDirectories) == 0 {
		cfg.PackageDirectories = []PackageDirectory{{Path: "."}}
	}
	if dependency {
		cfg.PackageDirectories = dependencyPackageDirectories(cfg.PackageDirectories)
	}

	p := Project{
		Root:               absRoot,
		Namespace:          cfg.Namespace,
		SourceAPIVersion:   cfg.SourceAPIVersion,
		PackageDirectories: cfg.PackageDirectories,
	}
	if cfgErr == nil {
		p.NamespaceRemaps = gladeCfg.Project.NamespaceRemaps
		p.ManagedPackageDependencies, p.DependencyDiagnostics = loadManagedPackageDependencies(gladeCfg.Project.ManagedPackageDependencies, gladeCfg.Project.NamespaceRemaps, stack)
		p.PackageShims, p.DependencyDiagnostics = loadPackageShims(gladeCfg.Project.PackageShims, stack, p.DependencyDiagnostics)
	}
	var dependencyCfg config.Config
	dependencyCfgOK := false
	if cfgErr != nil {
		if nearestCfg, _, err := config.LoadNearest(absRoot); err == nil {
			dependencyCfg = nearestCfg
			dependencyCfgOK = true
		}
	}
	p.ManagedPackageDependencies, p.DependencyDiagnostics = loadLocalSFDXPackageDependencies(absRoot, cfg, p, stack, dependencyCfg, dependencyCfgOK)

	includeConventionalRoots := !dependency && !cfg.HasManifest
	for _, pkgRoot := range packageRoots(absRoot, p.PackageDirectories, includeConventionalRoots) {
		if _, err := os.Stat(pkgRoot); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return Project{}, err
		}
		if err := collectFiles(pkgRoot, &p); err != nil {
			return Project{}, err
		}
	}

	dedupeProjectFiles(&p)

	sort.Strings(p.ApexFiles)
	sort.Strings(p.ObjectFiles)
	sort.Strings(p.FieldFiles)
	sort.Strings(p.FieldSetFiles)
	sort.Strings(p.RecordTypeFiles)
	sort.Strings(p.ValidationRuleFiles)
	sort.Strings(p.LabelFiles)
	sort.Strings(p.TranslationFiles)
	sort.Strings(p.StaticResourceFiles)
	sort.Strings(p.StaticResourceMetas)
	sort.Strings(p.DataCategoryGroupFiles)
	sort.Strings(p.DataWeaveFiles)
	sort.Strings(p.DataWeaveMetas)
	sort.Strings(p.ContentAssetFiles)
	sort.Strings(p.ContentAssetMetas)
	sort.Strings(p.EmailTemplateFiles)
	sort.Strings(p.FolderFiles)
	sort.Strings(p.NamedCredentialFiles)
	sort.Strings(p.RemoteSiteFiles)
	sort.Strings(p.CustomMetadataFiles)
	sort.Strings(p.WorkflowFiles)
	sort.Strings(p.FlowFiles)
	sort.Strings(p.ProfileFiles)
	sort.Strings(p.PermissionSetFiles)
	sort.Strings(p.PermissionSetGroupFiles)
	sort.Strings(p.PermissionAssignmentFiles)
	sort.Strings(p.ListViewFiles)
	sort.Strings(p.LayoutFiles)
	sort.Strings(p.CompactLayoutFiles)
	sort.Strings(p.TabFiles)
	sort.Strings(p.WebLinkFiles)
	sort.Strings(p.QuickActionFiles)
	sort.Strings(p.GlobalValueSetFiles)
	sort.Strings(p.StandardValueSetFiles)
	sort.Strings(p.FlexiPageFiles)
	sort.Strings(p.ApplicationFiles)
	sort.Strings(p.VisualforcePageFiles)
	p.VisualforcePageFiles = normalizeVisualforcePageFiles(p.VisualforcePageFiles)
	sort.Strings(p.VisualforceComponentFiles)
	sort.Strings(p.AuraFiles)
	sort.Strings(p.LWCFiles)
	sort.Strings(p.LWCHTMLFiles)
	sort.Strings(p.LWCCSSFiles)
	sort.Strings(p.LWCMetaFiles)
	return p, nil
}

func dedupeProjectFiles(p *Project) {
	p.ApexFiles = dedupeFilePaths(p.ApexFiles)
	p.ObjectFiles = dedupeFilePaths(p.ObjectFiles)
	p.FieldFiles = dedupeFilePaths(p.FieldFiles)
	p.FieldSetFiles = dedupeFilePaths(p.FieldSetFiles)
	p.RecordTypeFiles = dedupeFilePaths(p.RecordTypeFiles)
	p.ValidationRuleFiles = dedupeFilePaths(p.ValidationRuleFiles)
	p.LabelFiles = dedupeFilePaths(p.LabelFiles)
	p.TranslationFiles = dedupeFilePaths(p.TranslationFiles)
	p.StaticResourceFiles = dedupeFilePaths(p.StaticResourceFiles)
	p.StaticResourceMetas = dedupeFilePaths(p.StaticResourceMetas)
	p.DataCategoryGroupFiles = dedupeFilePaths(p.DataCategoryGroupFiles)
	p.DataWeaveFiles = dedupeFilePaths(p.DataWeaveFiles)
	p.DataWeaveMetas = dedupeFilePaths(p.DataWeaveMetas)
	p.ContentAssetFiles = dedupeFilePaths(p.ContentAssetFiles)
	p.ContentAssetMetas = dedupeFilePaths(p.ContentAssetMetas)
	p.EmailTemplateFiles = dedupeFilePaths(p.EmailTemplateFiles)
	p.FolderFiles = dedupeFilePaths(p.FolderFiles)
	p.NamedCredentialFiles = dedupeFilePaths(p.NamedCredentialFiles)
	p.RemoteSiteFiles = dedupeFilePaths(p.RemoteSiteFiles)
	p.CustomMetadataFiles = dedupeFilePaths(p.CustomMetadataFiles)
	p.WorkflowFiles = dedupeFilePaths(p.WorkflowFiles)
	p.FlowFiles = dedupeFilePaths(p.FlowFiles)
	p.ProfileFiles = dedupeFilePaths(p.ProfileFiles)
	p.PermissionSetFiles = dedupeFilePaths(p.PermissionSetFiles)
	p.PermissionSetGroupFiles = dedupeFilePaths(p.PermissionSetGroupFiles)
	p.PermissionAssignmentFiles = dedupeFilePaths(p.PermissionAssignmentFiles)
	p.ListViewFiles = dedupeFilePaths(p.ListViewFiles)
	p.LayoutFiles = dedupeFilePaths(p.LayoutFiles)
	p.CompactLayoutFiles = dedupeFilePaths(p.CompactLayoutFiles)
	p.TabFiles = dedupeFilePaths(p.TabFiles)
	p.WebLinkFiles = dedupeFilePaths(p.WebLinkFiles)
	p.QuickActionFiles = dedupeFilePaths(p.QuickActionFiles)
	p.GlobalValueSetFiles = dedupeFilePaths(p.GlobalValueSetFiles)
	p.StandardValueSetFiles = dedupeFilePaths(p.StandardValueSetFiles)
	p.FlexiPageFiles = dedupeFilePaths(p.FlexiPageFiles)
	p.ApplicationFiles = dedupeFilePaths(p.ApplicationFiles)
	p.VisualforcePageFiles = dedupeFilePaths(p.VisualforcePageFiles)
	p.VisualforceComponentFiles = dedupeFilePaths(p.VisualforceComponentFiles)
	p.AuraFiles = dedupeFilePaths(p.AuraFiles)
	p.LWCFiles = dedupeFilePaths(p.LWCFiles)
	p.LWCHTMLFiles = dedupeFilePaths(p.LWCHTMLFiles)
	p.LWCCSSFiles = dedupeFilePaths(p.LWCCSSFiles)
	p.LWCMetaFiles = dedupeFilePaths(p.LWCMetaFiles)
}

func dedupeFilePaths(paths []string) []string {
	if len(paths) < 2 {
		return paths
	}
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		key := canonicalFileKey(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, path)
	}
	return out
}

func canonicalFileKey(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved)
	}
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func packageDirectoriesFromConfig(paths []string) []PackageDirectory {
	out := make([]PackageDirectory, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		out = append(out, PackageDirectory{Path: filepath.ToSlash(path)})
	}
	return out
}

func dependencyPackageDirectories(dirs []PackageDirectory) []PackageDirectory {
	var defaults []PackageDirectory
	for _, dir := range dirs {
		if dir.Default {
			defaults = append(defaults, dir)
		}
	}
	if len(defaults) > 0 {
		return defaults
	}
	return dirs
}

func loadManagedPackageDependencies(configured []config.ManagedPackageDependency, remaps []namespaceremap.Rule, stack map[string]bool) ([]ManagedPackageDependency, []DependencyDiagnostic) {
	deps := make([]ManagedPackageDependency, 0, len(configured))
	var diagnostics []DependencyDiagnostic
	for _, dep := range configured {
		projectDep := ManagedPackageDependency{
			Namespace:    dep.Namespace,
			SourceRoot:   dep.SourceRoot,
			ArtifactPath: dep.ArtifactPath,
			Version:      dep.Version,
		}
		if dep.Namespace == "" || (dep.SourceRoot == "" && dep.ArtifactPath == "") {
			projectDep.Status = "invalid"
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  dep.Namespace,
				SourceRoot: dep.SourceRoot,
				Version:    dep.Version,
				Status:     "invalid",
				Code:       "dependency_invalid",
				Message:    "managed package dependency requires namespace and sourceRoot or artifactPath",
			})
			deps = append(deps, projectDep)
			continue
		}
		if dep.ArtifactPath != "" {
			if err := loadManagedPackageArtifactMetadata(dep.ArtifactPath, &projectDep); err != nil {
				status, code := managedPackageArtifactDiagnostic(err)
				projectDep.Status = status
				diagnostics = append(diagnostics, DependencyDiagnostic{
					Namespace:  dep.Namespace,
					SourceRoot: dep.ArtifactPath,
					Version:    dep.Version,
					Status:     status,
					Code:       code,
					Message:    err.Error(),
				})
				deps = append(deps, projectDep)
				continue
			}
			projectDep.Status = "loaded"
			deps = append(deps, projectDep)
			continue
		}
		info, err := os.Stat(dep.SourceRoot)
		if err != nil || !info.IsDir() {
			projectDep.Status = "missing"
			message := "managed package dependency source root not found"
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				message = err.Error()
			}
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  dep.Namespace,
				SourceRoot: dep.SourceRoot,
				Version:    dep.Version,
				Status:     "missing",
				Code:       "dependency_missing",
				Message:    message,
			})
			deps = append(deps, projectDep)
			continue
		}
		loaded, err := load(dep.SourceRoot, stack, true)
		if err != nil {
			projectDep.Status = "load_error"
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  dep.Namespace,
				SourceRoot: dep.SourceRoot,
				Version:    dep.Version,
				Status:     "load_error",
				Code:       "dependency_load_error",
				Message:    err.Error(),
			})
			deps = append(deps, projectDep)
			continue
		}
		loadedSourceNamespace := loaded.Namespace
		matchedRemaps := matchingNamespaceRemaps(remaps, loadedSourceNamespace, dep.Namespace)
		if sourceNamespaceRemapRequired(loadedSourceNamespace, dep.Namespace) && len(matchedRemaps) == 0 {
			projectDep.Status = "namespace_mismatch"
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  dep.Namespace,
				SourceRoot: dep.SourceRoot,
				Version:    dep.Version,
				Status:     "namespace_mismatch",
				Code:       "dependency_namespace_remap_missing",
				Message:    fmt.Sprintf("managed package dependency source namespace %q differs from configured namespace %q; add project.namespaceRemaps: [\"%s:%s\"]", loadedSourceNamespace, dep.Namespace, loadedSourceNamespace, dep.Namespace),
			})
			deps = append(deps, projectDep)
			continue
		}
		loaded.NamespaceRemaps = appendNamespaceRemaps(loaded.NamespaceRemaps, matchedRemaps...)
		loaded.Namespace = dep.Namespace
		projectDep.Project = &loaded
		projectDep.Status = "loaded"
		deps = append(deps, projectDep)
	}
	return deps, diagnostics
}

func loadLocalSFDXPackageDependencies(root string, cfg sfdxProject, p Project, stack map[string]bool, dependencyCfg config.Config, dependencyCfgOK bool) ([]ManagedPackageDependency, []DependencyDiagnostic) {
	deps := append([]ManagedPackageDependency(nil), p.ManagedPackageDependencies...)
	diagnostics := append([]DependencyDiagnostic(nil), p.DependencyDiagnostics...)
	packageNames := sfdxPackageDependencyNames(cfg)
	if dependencyCfgOK {
		configured := matchingConfiguredManagedPackageDependencies(packageNames, dependencyCfg.Project.ManagedPackageDependencies)
		configDeps, configDiagnostics := loadManagedPackageDependencies(configured, dependencyCfg.Project.NamespaceRemaps, stack)
		for _, dep := range configDeps {
			if managedPackageDependencySourceLoaded(deps, dep.SourceRoot) || managedPackageDependencyNamespaceLoaded(deps, dep.Namespace) {
				continue
			}
			deps = append(deps, dep)
		}
		diagnostics = append(diagnostics, configDiagnostics...)
	}
	for _, packageName := range packageNames {
		if managedPackageDependencyNamespaceLoaded(deps, packageName) {
			continue
		}
		depRoots := findLocalSFDXPackageDependencyRoots(root, packageName, dependencyCfgOK)
		if len(depRoots) == 0 {
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace: packageName,
				Status:    "missing",
				Code:      "dependency_missing",
				Message:   "declared SFDX package dependency has no configured source or artifact",
			})
			continue
		}
		if len(depRoots) > 1 {
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace: packageName,
				Status:    "ambiguous",
				Code:      "dependency_ambiguous",
				Message:   "declared SFDX package dependency matches multiple local projects: " + strings.Join(depRoots, ", "),
			})
			continue
		}
		depRoot := depRoots[0]
		if managedPackageDependencySourceLoaded(deps, depRoot) {
			continue
		}
		loaded, err := load(depRoot, stack, true)
		if err != nil {
			diagnostics = append(diagnostics, DependencyDiagnostic{
				SourceRoot: depRoot,
				Status:     "load_error",
				Code:       "dependency_load_error",
				Message:    err.Error(),
			})
			continue
		}
		if loaded.Root == "" {
			continue
		}
		namespace := loaded.Namespace
		if namespace == "" {
			namespace = p.Namespace
		}
		deps = append(deps, ManagedPackageDependency{
			Namespace:  namespace,
			SourceRoot: depRoot,
			Project:    &loaded,
			Status:     "loaded",
		})
	}
	for _, depRoot := range findReferencedNamespaceSiblingSFDXPackageDependencyRoots(root, packageNames, p, true) {
		if sameFilePath(depRoot, root) || managedPackageDependencySourceLoaded(deps, depRoot) {
			continue
		}
		loaded, err := load(depRoot, stack, true)
		if err != nil {
			diagnostics = append(diagnostics, DependencyDiagnostic{
				SourceRoot: depRoot,
				Status:     "load_error",
				Code:       "dependency_load_error",
				Message:    err.Error(),
			})
			continue
		}
		if loaded.Root == "" || loaded.Namespace == "" || managedPackageDependencyNamespaceLoaded(deps, loaded.Namespace) {
			continue
		}
		deps = append(deps, ManagedPackageDependency{
			Namespace:  loaded.Namespace,
			SourceRoot: depRoot,
			Project:    &loaded,
			Status:     "loaded",
		})
	}
	return deps, diagnostics
}

func matchingConfiguredManagedPackageDependencies(packageNames []string, configured []config.ManagedPackageDependency) []config.ManagedPackageDependency {
	wanted := make(map[string]bool, len(packageNames))
	for _, name := range packageNames {
		if normalized := normalizeSFDXPackageName(name); normalized != "" {
			wanted[normalized] = true
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	matched := make([]config.ManagedPackageDependency, 0, len(configured))
	for _, dep := range configured {
		if wanted[normalizeSFDXPackageName(dep.Namespace)] {
			matched = append(matched, dep)
		}
	}
	return matched
}

func sfdxPackageDependencyNames(cfg sfdxProject) []string {
	seen := make(map[string]bool)
	var names []string
	for _, dir := range cfg.PackageDirectories {
		for _, dep := range dir.Dependencies {
			name := normalizeSFDXPackageName(dep.Package)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, dep.Package)
		}
	}
	return names
}

func findLocalSFDXPackageDependencyRoots(root, packageName string, allowSiblingScan bool) []string {
	wanted := normalizeSFDXPackageName(packageName)
	if wanted == "" {
		return nil
	}
	root = filepath.Clean(root)
	seen := make(map[string]bool)
	var matches []string
	for _, candidate := range localSFDXPackageDependencyCandidates(root, packageName, allowSiblingScan) {
		if sameFilePath(candidate, root) || seen[candidate] {
			continue
		}
		seen[candidate] = true
		cfg, err := loadSFDXProject(candidate)
		if err != nil || !sfdxProjectDeclaresPackage(cfg, wanted) {
			continue
		}
		duplicate := false
		for _, match := range matches {
			if sameFilePath(match, candidate) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		matches = append(matches, candidate)
	}
	sort.Strings(matches)
	return matches
}

func findReferencedNamespaceSiblingSFDXPackageDependencyRoots(root string, packageNames []string, p Project, allowSiblingScan bool) []string {
	if !allowSiblingScan {
		return nil
	}
	unresolved := make(map[string]bool, len(packageNames))
	for _, packageName := range packageNames {
		if normalized := normalizeSFDXPackageName(packageName); normalized != "" {
			unresolved[normalized] = true
		}
	}
	if len(unresolved) == 0 {
		return nil
	}
	parent := filepath.Dir(filepath.Clean(root))
	var roots []string
	for _, candidate := range localSFDXSiblingProjectCandidates(parent) {
		if sameFilePath(candidate, root) {
			continue
		}
		cfg, err := loadSFDXProject(candidate)
		if err != nil {
			continue
		}
		for _, dir := range cfg.PackageDirectories {
			delete(unresolved, normalizeSFDXPackageName(dir.Package))
		}
		namespace := strings.TrimSpace(cfg.Namespace)
		if namespace == "" || strings.EqualFold(namespace, p.Namespace) || !projectRootReferencesNamespaceToken(root, p, namespace) {
			continue
		}
		roots = append(roots, candidate)
	}
	return roots
}

func localSFDXPackageDependencyCandidates(root, packageName string, allowSiblingScan bool) []string {
	seen := make(map[string]bool)
	var candidates []string
	add := func(dir string) {
		dir = filepath.Clean(dir)
		if dir == "." || dir == string(filepath.Separator) || seen[dir] {
			return
		}
		seen[dir] = true
		candidates = append(candidates, dir)
	}
	parent := filepath.Dir(filepath.Clean(root))
	grandparent := filepath.Dir(parent)
	for _, base := range []string{parent, grandparent} {
		if base == "" || base == "." {
			continue
		}
		add(base)
		for _, name := range localSFDXPackageDependencyDirNames(packageName) {
			if name == "" {
				continue
			}
			add(filepath.Join(base, name))
			add(filepath.Join(base, "packages", name))
		}
	}
	if allowSiblingScan {
		for _, candidate := range localSFDXSiblingProjectCandidates(parent) {
			add(candidate)
		}
	}
	return candidates
}

func localSFDXSiblingProjectCandidates(parent string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		candidate := filepath.Join(parent, entry.Name())
		info, err := os.Stat(filepath.Join(candidate, "sfdx-project.json"))
		if err == nil && !info.IsDir() {
			candidates = append(candidates, candidate)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func localSFDXPackageDependencyDirNames(packageName string) []string {
	raw := strings.TrimSpace(packageName)
	if i := strings.Index(raw, "@"); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	normalized := normalizeSFDXPackageName(raw)
	var names []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, `/\`) {
			return
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}
	add(normalized)
	add(raw)
	add(strings.ReplaceAll(normalized, " ", "-"))
	add(strings.ReplaceAll(normalized, " ", "_"))
	add(strings.ReplaceAll(normalized, " ", ""))
	return names
}

func sfdxProjectDeclaresPackage(cfg sfdxProject, wanted string) bool {
	for _, dir := range cfg.PackageDirectories {
		if normalizeSFDXPackageName(dir.Package) == wanted {
			return true
		}
	}
	return false
}

func projectRootReferencesNamespaceToken(root string, p Project, namespace string) bool {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return false
	}
	token := namespace + "__"
	for _, pkgRoot := range packageRoots(root, p.PackageDirectories, true) {
		if referencesNamespaceTokenInRoot(pkgRoot, token) {
			return true
		}
	}
	return false
}

func referencesNamespaceTokenInRoot(root, token string) bool {
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !namespaceReferenceCandidatePath(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), token) {
			return errFoundNamespaceReference
		}
		return nil
	})
	return errors.Is(err, errFoundNamespaceReference)
}

var errFoundNamespaceReference = errors.New("found namespace reference")

func namespaceReferenceCandidatePath(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range []string{
		".cls",
		".trigger",
		".object",
		".object-meta.xml",
		".field-meta.xml",
		".fieldset-meta.xml",
		".recordtype-meta.xml",
		".validationrule-meta.xml",
		".layout-meta.xml",
		".permissionset-meta.xml",
		".profile-meta.xml",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func normalizeSFDXPackageName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func managedPackageDependencySourceLoaded(deps []ManagedPackageDependency, sourceRoot string) bool {
	for _, dep := range deps {
		if dep.SourceRoot != "" && sameFilePath(dep.SourceRoot, sourceRoot) {
			return true
		}
	}
	return false
}

func managedPackageDependencyNamespaceLoaded(deps []ManagedPackageDependency, namespace string) bool {
	namespace = normalizeSFDXPackageName(namespace)
	if namespace == "" {
		return false
	}
	for _, dep := range deps {
		if normalizeSFDXPackageName(dep.Namespace) == namespace {
			return true
		}
	}
	return false
}

func sameFilePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if leftInfo, leftErr := os.Stat(left); leftErr == nil {
		if rightInfo, rightErr := os.Stat(right); rightErr == nil && os.SameFile(leftInfo, rightInfo) {
			return true
		}
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func matchingNamespaceRemaps(rules []namespaceremap.Rule, loadedNamespace, configuredNamespace string) []namespaceremap.Rule {
	var matched []namespaceremap.Rule
	for _, rule := range rules {
		if strings.EqualFold(loadedNamespace, rule.From) && strings.EqualFold(configuredNamespace, rule.To) {
			matched = append(matched, rule)
		}
	}
	return matched
}

func sourceNamespaceRemapRequired(loadedNamespace, configuredNamespace string) bool {
	loadedNamespace = strings.TrimSpace(loadedNamespace)
	configuredNamespace = strings.TrimSpace(configuredNamespace)
	return loadedNamespace != "" && configuredNamespace != "" && !strings.EqualFold(loadedNamespace, configuredNamespace)
}

func appendNamespaceRemaps(base []namespaceremap.Rule, extra ...namespaceremap.Rule) []namespaceremap.Rule {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(extra))
	for _, rule := range base {
		seen[namespaceRemapKey(rule)] = true
	}
	for _, rule := range extra {
		key := namespaceRemapKey(rule)
		if seen[key] {
			continue
		}
		seen[key] = true
		base = append(base, rule)
	}
	return base
}

func namespaceRemapKey(rule namespaceremap.Rule) string {
	return strings.ToLower(rule.From) + "\x00" + strings.ToLower(rule.To)
}

func loadPackageShims(configured []config.PackageShim, stack map[string]bool, diagnostics []DependencyDiagnostic) ([]PackageShim, []DependencyDiagnostic) {
	shims := make([]PackageShim, 0, len(configured))
	for _, shim := range configured {
		projectShim := PackageShim{
			Namespace:  shim.Namespace,
			SourceRoot: shim.SourceRoot,
		}
		if shim.Namespace == "" || shim.SourceRoot == "" {
			projectShim.Status = "invalid"
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  shim.Namespace,
				SourceRoot: shim.SourceRoot,
				Status:     "invalid",
				Code:       "package_shim_invalid",
				Message:    "package shim requires namespace and sourceRoot",
			})
			shims = append(shims, projectShim)
			continue
		}
		info, err := os.Stat(shim.SourceRoot)
		if err != nil || !info.IsDir() {
			projectShim.Status = "missing"
			message := "package shim source root not found"
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				message = err.Error()
			}
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  shim.Namespace,
				SourceRoot: shim.SourceRoot,
				Status:     "missing",
				Code:       "package_shim_missing",
				Message:    message,
			})
			shims = append(shims, projectShim)
			continue
		}
		loaded, err := load(shim.SourceRoot, stack, true)
		if err != nil {
			projectShim.Status = "load_error"
			diagnostics = append(diagnostics, DependencyDiagnostic{
				Namespace:  shim.Namespace,
				SourceRoot: shim.SourceRoot,
				Status:     "load_error",
				Code:       "package_shim_load_error",
				Message:    err.Error(),
			})
			shims = append(shims, projectShim)
			continue
		}
		loaded.Namespace = shim.Namespace
		projectShim.Project = &loaded
		projectShim.Status = "loaded"
		shims = append(shims, projectShim)
	}
	return shims, diagnostics
}

type managedPackageArtifactError struct {
	status  string
	code    string
	message string
}

type managedPackageArtifactMetadata struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Namespace        string `json:"namespace"`
	Version          string `json:"version"`
	SourceHash       string `json:"sourceHash"`
	SourceAPIVersion string `json:"sourceApiVersion"`
	ApexTypes        []struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"apexTypes"`
	Objects []struct {
		Name string `json:"name"`
	} `json:"objects"`
}

func (e managedPackageArtifactError) Error() string {
	return e.message
}

func newManagedPackageArtifactError(status, code, message string) error {
	return managedPackageArtifactError{status: status, code: code, message: message}
}

func managedPackageArtifactDiagnostic(err error) (string, string) {
	var artifactErr managedPackageArtifactError
	if errors.As(err, &artifactErr) {
		return artifactErr.status, artifactErr.code
	}
	if errors.Is(err, os.ErrNotExist) {
		return "missing", "dependency_missing"
	}
	return "load_error", "dependency_load_error"
}

func loadManagedPackageArtifactMetadata(path string, dep *ManagedPackageDependency) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newManagedPackageArtifactError("missing", "dependency_missing", "managed package dependency artifact not found")
		}
		return newManagedPackageArtifactError("load_error", "dependency_load_error", err.Error())
	}
	if info.IsDir() {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact path is a directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", err.Error())
	}
	var metadata managedPackageArtifactMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", err.Error())
	}
	if issues := validateManagedPackageArtifactMetadata(metadata); len(issues) > 0 {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact invalid: "+strings.Join(issues, "; "))
	}
	if strings.TrimSpace(metadata.Namespace) == "" {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact namespace is required")
	}
	if !strings.EqualFold(metadata.Namespace, dep.Namespace) {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact namespace does not match configured namespace")
	}
	if dep.Version != "" && strings.TrimSpace(metadata.Version) == "" {
		return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact version is required when config pins a version")
	}
	if dep.Version != "" && dep.Version != metadata.Version {
		return newManagedPackageArtifactError("version_mismatch", "dependency_version_mismatch", fmt.Sprintf("managed package dependency artifact version %q does not match configured version %q", metadata.Version, dep.Version))
	}
	if dep.Version == "" {
		dep.Version = metadata.Version
	}
	return nil
}

func validateManagedPackageArtifactMetadata(metadata managedPackageArtifactMetadata) []string {
	issues := make([]string, 0)
	if metadata.SchemaVersion > 2 {
		issues = append(issues, fmt.Sprintf("unsupported artifact schemaVersion %d", metadata.SchemaVersion))
	}
	if strings.TrimSpace(metadata.Namespace) == "" {
		issues = append(issues, "namespace is required")
	}
	if strings.TrimSpace(metadata.SourceHash) == "" {
		issues = append(issues, "sourceHash is required")
	}
	if metadata.SchemaVersion == 2 {
		if strings.TrimSpace(metadata.SourceAPIVersion) == "" {
			issues = append(issues, "sourceApiVersion is required")
		} else if _, err := apexversion.ResolveSource(metadata.SourceAPIVersion); err != nil {
			issues = append(issues, err.Error())
		}
	}
	for _, typ := range metadata.ApexTypes {
		if strings.TrimSpace(typ.Name) == "" {
			issues = append(issues, "apex type name is required")
		}
		if strings.TrimSpace(typ.Namespace) == "" {
			issues = append(issues, "apex type "+typ.Name+" is missing namespace")
		}
	}
	for _, object := range metadata.Objects {
		if strings.TrimSpace(object.Name) == "" {
			issues = append(issues, "object name is required")
		}
	}
	return issues
}

func loadSFDXProject(root string) (sfdxProject, error) {
	path := filepath.Join(root, "sfdx-project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sfdxProject{PackageDirectories: []PackageDirectory{{Path: "."}}}, nil
		}
		return sfdxProject{}, err
	}

	var cfg sfdxProject
	if err := json.Unmarshal(data, &cfg); err != nil {
		return sfdxProject{}, err
	}
	cfg.HasManifest = true
	return cfg, nil
}

func packageRoots(root string, packageDirs []PackageDirectory, includeConventionalRoots bool) []string {
	seen := make(map[string]bool)
	var roots []string
	add := func(rel string) {
		if rel == "" {
			return
		}
		abs := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
		if seen[abs] {
			return
		}
		seen[abs] = true
		roots = append(roots, abs)
	}
	for _, pkg := range packageDirs {
		add(pkg.Path)
	}
	if !includeConventionalRoots {
		return roots
	}
	for _, rel := range []string{"src", "force-app", "sfdx-source", "unpackaged"} {
		abs := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
		if hasCoveredRoot(abs, roots) {
			continue
		}
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			add(rel)
		}
	}
	return roots
}

func hasCoveredRoot(root string, roots []string) bool {
	for _, existing := range roots {
		if existing == root || strings.HasPrefix(existing, root+string(filepath.Separator)) || strings.HasPrefix(root, existing+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func collectFiles(root string, p *Project) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		lower := strings.ToLower(path)
		switch {
		case isStaticResourceContentFile(path):
			p.StaticResourceFiles = append(p.StaticResourceFiles, path)
		case strings.HasSuffix(lower, ".cls"), strings.HasSuffix(lower, ".trigger"):
			p.ApexFiles = append(p.ApexFiles, path)
		case strings.HasSuffix(lower, ".object-meta.xml"), strings.HasSuffix(lower, ".object") && isLegacyObjectPath(lower):
			p.ObjectFiles = append(p.ObjectFiles, path)
		case strings.HasSuffix(lower, ".field-meta.xml"):
			p.FieldFiles = append(p.FieldFiles, path)
		case strings.HasSuffix(lower, ".fieldset-meta.xml"):
			p.FieldSetFiles = append(p.FieldSetFiles, path)
		case strings.HasSuffix(lower, ".recordtype-meta.xml"):
			p.RecordTypeFiles = append(p.RecordTypeFiles, path)
		case strings.HasSuffix(lower, ".validationrule-meta.xml"):
			p.ValidationRuleFiles = append(p.ValidationRuleFiles, path)
		case strings.HasSuffix(lower, ".labels"), strings.HasSuffix(lower, ".labels-meta.xml"):
			p.LabelFiles = append(p.LabelFiles, path)
		case strings.HasSuffix(lower, ".translation"), strings.HasSuffix(lower, ".translation-meta.xml"):
			p.TranslationFiles = append(p.TranslationFiles, path)
		case strings.HasSuffix(lower, ".resource-meta.xml"), strings.HasSuffix(lower, ".staticresource-meta.xml"):
			p.StaticResourceMetas = append(p.StaticResourceMetas, path)
		case strings.HasSuffix(lower, ".datacategorygroup-meta.xml"):
			p.DataCategoryGroupFiles = append(p.DataCategoryGroupFiles, path)
		case strings.HasSuffix(lower, ".dwl-meta.xml"):
			p.DataWeaveMetas = append(p.DataWeaveMetas, path)
		case strings.HasSuffix(lower, ".dwl"):
			p.DataWeaveFiles = append(p.DataWeaveFiles, path)
		case strings.HasSuffix(lower, ".resource"):
			p.StaticResourceFiles = append(p.StaticResourceFiles, path)
		case strings.HasSuffix(lower, ".asset-meta.xml"):
			p.ContentAssetMetas = append(p.ContentAssetMetas, path)
		case strings.HasSuffix(lower, ".asset"):
			p.ContentAssetFiles = append(p.ContentAssetFiles, path)
		case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
			p.EmailTemplateFiles = append(p.EmailTemplateFiles, path)
		case isFolderMetadataPath(lower):
			p.FolderFiles = append(p.FolderFiles, path)
		case strings.HasSuffix(lower, ".namedcredential"), strings.HasSuffix(lower, ".namedcredential-meta.xml"):
			p.NamedCredentialFiles = append(p.NamedCredentialFiles, path)
		case strings.HasSuffix(lower, ".remotesite"), strings.HasSuffix(lower, ".remotesite-meta.xml"):
			p.RemoteSiteFiles = append(p.RemoteSiteFiles, path)
		case strings.HasSuffix(lower, ".md-meta.xml"), strings.HasSuffix(lower, ".md") && isCustomMetadataPath(lower):
			p.CustomMetadataFiles = append(p.CustomMetadataFiles, path)
		case strings.HasSuffix(lower, ".workflow-meta.xml"), strings.HasSuffix(lower, ".workflow"):
			p.WorkflowFiles = append(p.WorkflowFiles, path)
		case strings.HasSuffix(lower, ".flow-meta.xml"), strings.HasSuffix(lower, ".flow"):
			p.FlowFiles = append(p.FlowFiles, path)
		case strings.HasSuffix(lower, ".profile"), strings.HasSuffix(lower, ".profile-meta.xml"):
			p.ProfileFiles = append(p.ProfileFiles, path)
		case strings.HasSuffix(lower, ".permissionset"), strings.HasSuffix(lower, ".permissionset-meta.xml"):
			p.PermissionSetFiles = append(p.PermissionSetFiles, path)
		case strings.HasSuffix(lower, ".permissionsetgroup"), strings.HasSuffix(lower, ".permissionsetgroup-meta.xml"):
			p.PermissionSetGroupFiles = append(p.PermissionSetGroupFiles, path)
		case strings.HasSuffix(lower, ".permissionsetassignment"), strings.HasSuffix(lower, ".permissionsetassignment-meta.xml"):
			p.PermissionAssignmentFiles = append(p.PermissionAssignmentFiles, path)
		case strings.HasSuffix(lower, ".listview-meta.xml"):
			p.ListViewFiles = append(p.ListViewFiles, path)
		case strings.HasSuffix(lower, ".layout-meta.xml"), strings.HasSuffix(lower, ".layout"):
			p.LayoutFiles = append(p.LayoutFiles, path)
		case strings.HasSuffix(lower, ".compactlayout-meta.xml"):
			p.CompactLayoutFiles = append(p.CompactLayoutFiles, path)
		case strings.HasSuffix(lower, ".tab"), strings.HasSuffix(lower, ".tab-meta.xml"):
			p.TabFiles = append(p.TabFiles, path)
		case strings.HasSuffix(lower, ".weblink"), strings.HasSuffix(lower, ".weblink-meta.xml"):
			p.WebLinkFiles = append(p.WebLinkFiles, path)
		case strings.HasSuffix(lower, ".quickaction"), strings.HasSuffix(lower, ".quickaction-meta.xml"):
			p.QuickActionFiles = append(p.QuickActionFiles, path)
		case strings.HasSuffix(lower, ".globalvalueset"), strings.HasSuffix(lower, ".globalvalueset-meta.xml"):
			p.GlobalValueSetFiles = append(p.GlobalValueSetFiles, path)
		case strings.HasSuffix(lower, ".standardvalueset"), strings.HasSuffix(lower, ".standardvalueset-meta.xml"):
			p.StandardValueSetFiles = append(p.StandardValueSetFiles, path)
		case strings.HasSuffix(lower, ".flexipage"), strings.HasSuffix(lower, ".flexipage-meta.xml"):
			p.FlexiPageFiles = append(p.FlexiPageFiles, path)
		case strings.HasSuffix(lower, ".app-meta.xml"), strings.HasSuffix(lower, ".app") && strings.Contains(filepath.ToSlash(lower), "/applications/"):
			p.ApplicationFiles = append(p.ApplicationFiles, path)
		case strings.HasSuffix(lower, ".page"), strings.HasSuffix(lower, ".page-meta.xml"):
			p.VisualforcePageFiles = append(p.VisualforcePageFiles, path)
		case strings.HasSuffix(lower, ".component"):
			p.VisualforceComponentFiles = append(p.VisualforceComponentFiles, path)
		case isAuraPath(lower) && isAuraSourceFile(lower):
			p.AuraFiles = append(p.AuraFiles, path)
		case isLWCPath(lower):
			switch {
			case strings.HasSuffix(lower, ".js"):
				p.LWCFiles = append(p.LWCFiles, path)
			case strings.HasSuffix(lower, ".html"):
				p.LWCHTMLFiles = append(p.LWCHTMLFiles, path)
			case strings.HasSuffix(lower, ".css"):
				p.LWCCSSFiles = append(p.LWCCSSFiles, path)
			case strings.HasSuffix(lower, ".js-meta.xml"):
				p.LWCMetaFiles = append(p.LWCMetaFiles, path)
			}
		}
		return nil
	})
}

func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case ".git", ".sfdx", ".sf", ".claude", "node_modules", ".idea", ".vscode", ".DS_Store", "__tests__":
		return true
	default:
		return false
	}
}

func isStaticResourceContentFile(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part != "staticresources" || i >= len(parts)-1 {
			continue
		}
		name := strings.ToLower(parts[len(parts)-1])
		if strings.HasSuffix(name, "-meta.xml") || strings.HasSuffix(name, ".xml") {
			return false
		}
		return true
	}
	return false
}

func isFolderMetadataPath(path string) bool {
	for _, suffix := range []string{
		".documentfolder-meta.xml",
		".emailfolder-meta.xml",
		".reportfolder-meta.xml",
		".dashboardfolder-meta.xml",
	} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func isCustomMetadataPath(path string) bool {
	slash := filepath.ToSlash(path)
	if strings.Contains(slash, "/custommetadata/") {
		return true
	}
	const marker = "/objects/"
	for idx := strings.Index(slash, marker); idx >= 0; {
		rest := slash[idx+len(marker):]
		next := strings.IndexByte(rest, '/')
		if next > 0 && strings.HasSuffix(rest[:next], "__mdt") && strings.HasPrefix(rest[next+1:], "records/") {
			return true
		}
		nextIdx := strings.Index(rest, marker)
		if nextIdx < 0 {
			break
		}
		idx += len(marker) + nextIdx
	}
	return false
}

func isLegacyObjectPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/objects/")
}

func isAuraPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/aura/")
}

func isLWCPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/lwc/")
}

func isAuraSourceFile(path string) bool {
	for _, suffix := range []string{
		".cmp", ".cmp-meta.xml",
		".app", ".app-meta.xml",
		".evt", ".evt-meta.xml",
		".intf", ".intf-meta.xml",
		".design", ".design-meta.xml",
		".css", ".svg", ".auradoc",
	} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".js")
}
