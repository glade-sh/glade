package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Project struct {
	Root                      string             `json:"root"`
	Namespace                 string             `json:"namespace,omitempty"`
	SourceAPIVersion          string             `json:"sourceApiVersion,omitempty"`
	PackageDirectories        []PackageDirectory `json:"packageDirectories"`
	ApexFiles                 []string           `json:"apexFiles"`
	ObjectFiles               []string           `json:"objectFiles"`
	FieldFiles                []string           `json:"fieldFiles"`
	FieldSetFiles             []string           `json:"fieldSetFiles"`
	RecordTypeFiles           []string           `json:"recordTypeFiles"`
	ValidationRuleFiles       []string           `json:"validationRuleFiles"`
	LabelFiles                []string           `json:"labelFiles"`
	TranslationFiles          []string           `json:"translationFiles,omitempty"`
	StaticResourceFiles       []string           `json:"staticResourceFiles"`
	StaticResourceMetas       []string           `json:"staticResourceMetas"`
	ContentAssetFiles         []string           `json:"contentAssetFiles,omitempty"`
	ContentAssetMetas         []string           `json:"contentAssetMetas,omitempty"`
	EmailTemplateFiles        []string           `json:"emailTemplateFiles,omitempty"`
	NamedCredentialFiles      []string           `json:"namedCredentialFiles"`
	RemoteSiteFiles           []string           `json:"remoteSiteFiles"`
	CustomMetadataFiles       []string           `json:"customMetadataFiles"`
	WorkflowFiles             []string           `json:"workflowFiles"`
	FlowFiles                 []string           `json:"flowFiles"`
	ProfileFiles              []string           `json:"profileFiles"`
	PermissionSetFiles        []string           `json:"permissionSetFiles"`
	PermissionAssignmentFiles []string           `json:"permissionAssignmentFiles"`
	ListViewFiles             []string           `json:"listViewFiles"`
	LayoutFiles               []string           `json:"layoutFiles"`
	CompactLayoutFiles        []string           `json:"compactLayoutFiles"`
	TabFiles                  []string           `json:"tabFiles"`
	WebLinkFiles              []string           `json:"webLinkFiles"`
	QuickActionFiles          []string           `json:"quickActionFiles"`
	GlobalValueSetFiles       []string           `json:"globalValueSetFiles"`
	StandardValueSetFiles     []string           `json:"standardValueSetFiles"`
	FlexiPageFiles            []string           `json:"flexiPageFiles"`
	ApplicationFiles          []string           `json:"applicationFiles"`
	VisualforcePageFiles      []string           `json:"visualforcePageFiles"`
	VisualforceComponentFiles []string           `json:"visualforceComponentFiles"`
	AuraFiles                 []string           `json:"auraFiles"`
	LWCFiles                  []string           `json:"lwcFiles"`
}

type PackageDirectory struct {
	Path    string `json:"path"`
	Default bool   `json:"default,omitempty"`
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
}

func Load(root string) (Project, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Project{}, err
	}

	cfg, err := loadSFDXProject(absRoot)
	if err != nil {
		return Project{}, err
	}
	if len(cfg.PackageDirectories) == 0 {
		cfg.PackageDirectories = []PackageDirectory{{Path: "."}}
	}

	p := Project{
		Root:               absRoot,
		Namespace:          cfg.Namespace,
		SourceAPIVersion:   cfg.SourceAPIVersion,
		PackageDirectories: cfg.PackageDirectories,
	}

	for _, pkgRoot := range packageRoots(absRoot, p.PackageDirectories) {
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
	sort.Strings(p.ContentAssetFiles)
	sort.Strings(p.ContentAssetMetas)
	sort.Strings(p.EmailTemplateFiles)
	sort.Strings(p.NamedCredentialFiles)
	sort.Strings(p.RemoteSiteFiles)
	sort.Strings(p.CustomMetadataFiles)
	sort.Strings(p.WorkflowFiles)
	sort.Strings(p.FlowFiles)
	sort.Strings(p.ProfileFiles)
	sort.Strings(p.PermissionSetFiles)
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
	sort.Strings(p.VisualforceComponentFiles)
	sort.Strings(p.AuraFiles)
	sort.Strings(p.LWCFiles)
	return p, nil
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
	return cfg, nil
}

func packageRoots(root string, packageDirs []PackageDirectory) []string {
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
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			if isStaticResourceVendorDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		lower := strings.ToLower(path)
		switch {
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
		case strings.HasSuffix(lower, ".resource"):
			p.StaticResourceFiles = append(p.StaticResourceFiles, path)
		case strings.HasSuffix(lower, ".asset-meta.xml"):
			p.ContentAssetMetas = append(p.ContentAssetMetas, path)
		case strings.HasSuffix(lower, ".asset"):
			p.ContentAssetFiles = append(p.ContentAssetFiles, path)
		case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
			p.EmailTemplateFiles = append(p.EmailTemplateFiles, path)
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
		case strings.HasSuffix(lower, ".page"):
			p.VisualforcePageFiles = append(p.VisualforcePageFiles, path)
		case strings.HasSuffix(lower, ".component"):
			p.VisualforceComponentFiles = append(p.VisualforceComponentFiles, path)
		case isAuraPath(lower) && isAuraSourceFile(lower):
			p.AuraFiles = append(p.AuraFiles, path)
		case isLWCPath(lower) && strings.HasSuffix(lower, ".js"):
			p.LWCFiles = append(p.LWCFiles, path)
		}
		return nil
	})
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".sfdx", ".sf", ".claude", "node_modules", ".idea", ".vscode", ".DS_Store", "__tests__":
		return true
	default:
		return false
	}
}

func isStaticResourceVendorDir(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "staticresources" && i < len(parts)-1 {
			return true
		}
	}
	return false
}

func isCustomMetadataPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/custommetadata/")
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
	for _, suffix := range []string{".cmp", ".app", ".evt", ".design"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	base := filepath.Base(path)
	return strings.HasSuffix(base, "controller.js") || strings.HasSuffix(base, "helper.js")
}
