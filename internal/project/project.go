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
	Root                string             `json:"root"`
	Namespace           string             `json:"namespace,omitempty"`
	SourceAPIVersion    string             `json:"sourceApiVersion,omitempty"`
	PackageDirectories  []PackageDirectory `json:"packageDirectories"`
	ApexFiles           []string           `json:"apexFiles"`
	ObjectFiles         []string           `json:"objectFiles"`
	FieldFiles          []string           `json:"fieldFiles"`
	RecordTypeFiles     []string           `json:"recordTypeFiles"`
	ValidationRuleFiles []string           `json:"validationRuleFiles"`
}

type PackageDirectory struct {
	Path    string `json:"path"`
	Default bool   `json:"default,omitempty"`
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

	for _, pkg := range p.PackageDirectories {
		pkgRoot := filepath.Join(absRoot, filepath.FromSlash(pkg.Path))
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
	sort.Strings(p.RecordTypeFiles)
	sort.Strings(p.ValidationRuleFiles)
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

func collectFiles(root string, p *Project) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		lower := strings.ToLower(path)
		switch {
		case strings.HasSuffix(lower, ".cls"), strings.HasSuffix(lower, ".trigger"):
			p.ApexFiles = append(p.ApexFiles, path)
		case strings.HasSuffix(lower, ".object-meta.xml"):
			p.ObjectFiles = append(p.ObjectFiles, path)
		case strings.HasSuffix(lower, ".field-meta.xml"):
			p.FieldFiles = append(p.FieldFiles, path)
		case strings.HasSuffix(lower, ".recordtype-meta.xml"):
			p.RecordTypeFiles = append(p.RecordTypeFiles, path)
		case strings.HasSuffix(lower, ".validationrule-meta.xml"):
			p.ValidationRuleFiles = append(p.ValidationRuleFiles, path)
		}
		return nil
	})
}
