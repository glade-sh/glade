package typesys

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
)

type Index struct {
	Project               ProjectInfo                   `json:"project"`
	Types                 []TypeSymbol                  `json:"types"`
	Triggers              []TriggerSymbol               `json:"triggers"`
	Objects               []schema.Object               `json:"objects"`
	CustomMetadataRecords []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	Dependencies          []DependencyInfo              `json:"dependencies,omitempty"`
	Diagnostics           []diagnostic.Diagnostic       `json:"diagnostics,omitempty"`
}

type ProjectInfo struct {
	Root             string `json:"root"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"sourceApiVersion,omitempty"`
}

type TypeSymbol struct {
	Kind       apexast.DeclarationKind `json:"kind"`
	Name       string                  `json:"name"`
	File       string                  `json:"file"`
	Namespace  string                  `json:"namespace,omitempty"`
	SourceRoot string                  `json:"sourceRoot,omitempty"`
	Version    string                  `json:"version,omitempty"`
	Dependency bool                    `json:"dependency,omitempty"`
	Modifiers  []string                `json:"modifiers,omitempty"`
	IsTest     bool                    `json:"isTest,omitempty"`
	SuperClass string                  `json:"superClass,omitempty"`
	Interfaces []string                `json:"interfaces,omitempty"`
	Range      diagnostic.Range        `json:"range"`
	Members    []MemberSymbol          `json:"members,omitempty"`
}

type MemberSymbol struct {
	Kind       apexast.DeclarationKind `json:"kind"`
	Name       string                  `json:"name"`
	Type       string                  `json:"type,omitempty"`
	Modifiers  []string                `json:"modifiers,omitempty"`
	Parameters []apexast.Parameter     `json:"parameters,omitempty"`
	Accessors  []apexast.Accessor      `json:"accessors,omitempty"`
	IsTest     bool                    `json:"isTest,omitempty"`
	Range      diagnostic.Range        `json:"range"`
}

type TriggerSymbol struct {
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace,omitempty"`
	ObjectName string           `json:"objectName"`
	Events     []string         `json:"events,omitempty"`
	File       string           `json:"file"`
	Dependency bool             `json:"dependency,omitempty"`
	Range      diagnostic.Range `json:"range"`
}

type DependencyInfo struct {
	Namespace       string                  `json:"namespace"`
	SourceRoot      string                  `json:"sourceRoot"`
	Version         string                  `json:"version,omitempty"`
	Status          string                  `json:"status"`
	ApexTypes       int                     `json:"apexTypes,omitempty"`
	Objects         int                     `json:"objects,omitempty"`
	Labels          int                     `json:"labels,omitempty"`
	StaticResources int                     `json:"staticResources,omitempty"`
	Diagnostics     []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

func Build(p project.Project, s schema.Schema) (idx Index) {
	parser := apexast.NewParser()
	idx = Index{
		Project: ProjectInfo{
			Root:             p.Root,
			Namespace:        p.Namespace,
			SourceAPIVersion: p.SourceAPIVersion,
		},
		Objects:               s.Objects,
		CustomMetadataRecords: s.CustomMetadataRecords,
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERTYPE000",
				Message:  fmt.Sprintf("internal type-index panic: %v", recovered),
			})
		}
	}()
	seenTypes := make(map[string][]seenTypeSymbol)
	for _, depDiag := range p.DependencyDiagnostics {
		idx.Dependencies = append(idx.Dependencies, DependencyInfo{
			Namespace:  depDiag.Namespace,
			SourceRoot: depDiag.SourceRoot,
			Version:    depDiag.Version,
			Status:     depDiag.Status,
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     depDiag.Code,
				Message:  depDiag.Message,
			}},
		})
		idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     depDiag.Code,
			Message:  depDiag.Message,
		})
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		depSchema, err := schema.LoadProject(*dep.Project)
		if err != nil {
			diag := diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "dependency_load_error",
				Message:  fmt.Sprintf("managed package dependency %s schema load failed: %v", dep.Namespace, err),
			}
			idx.Diagnostics = append(idx.Diagnostics, diag)
			idx.Dependencies = append(idx.Dependencies, DependencyInfo{
				Namespace:   dep.Namespace,
				SourceRoot:  dep.SourceRoot,
				Version:     dep.Version,
				Status:      "load_error",
				Diagnostics: []diagnostic.Diagnostic{diag},
			})
			continue
		}
		beforeTypes := len(idx.Types)
		appendProjectSymbols(&idx, parser, *dep.Project, true, dep.Namespace, dep.Version, seenTypes)
		idx.Objects = append(idx.Objects, depSchema.Objects...)
		idx.CustomMetadataRecords = append(idx.CustomMetadataRecords, depSchema.CustomMetadataRecords...)
		idx.Dependencies = append(idx.Dependencies, DependencyInfo{
			Namespace:       dep.Namespace,
			SourceRoot:      dep.SourceRoot,
			Version:         dep.Version,
			Status:          dep.Status,
			ApexTypes:       len(idx.Types) - beforeTypes,
			Objects:         len(depSchema.Objects),
			Labels:          len(dep.Project.LabelFiles),
			StaticResources: len(dep.Project.StaticResourceFiles) + len(dep.Project.StaticResourceMetas),
		})
	}
	appendProjectSymbols(&idx, parser, p, false, p.Namespace, "", seenTypes)

	sort.Slice(idx.Types, func(i, j int) bool {
		if idx.Types[i].Namespace == idx.Types[j].Namespace {
			return idx.Types[i].Name < idx.Types[j].Name
		}
		return idx.Types[i].Namespace < idx.Types[j].Namespace
	})
	sort.Slice(idx.Triggers, func(i, j int) bool {
		if idx.Triggers[i].Namespace == idx.Triggers[j].Namespace {
			return idx.Triggers[i].Name < idx.Triggers[j].Name
		}
		return idx.Triggers[i].Namespace < idx.Triggers[j].Namespace
	})
	return idx
}

func appendProjectSymbols(idx *Index, parser *apexast.Parser, p project.Project, dependency bool, namespace, version string, seenTypes map[string][]seenTypeSymbol) {
	for _, path := range p.ApexFiles {
		file, err := parser.ParseFile(path)
		if err != nil {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERTYPE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		idx.Diagnostics = append(idx.Diagnostics, file.Diagnostics...)
		if len(file.Diagnostics) > 0 {
			continue
		}
		for _, decl := range file.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
				if source, err := os.ReadFile(path); err == nil {
					for _, sym := range typeSymbolsFromDeclaration(path, decl, "", string(source)) {
						sym.Namespace = namespace
						sym.SourceRoot = p.Root
						sym.Version = version
						sym.Dependency = dependency
						key := namespaceTypeKey(sym.Namespace, sym.Name)
						currentPackage := p.PackagePathForFile(path)
						if previous, ok := conflictingSeenType(seenTypes[key], currentPackage); ok {
							idx.Diagnostics = append(idx.Diagnostics, duplicateDiagnostic(sym, previous.Symbol))
						} else {
							seenTypes[key] = append(seenTypes[key], seenTypeSymbol{Symbol: sym, PackagePath: currentPackage})
						}
						idx.Types = append(idx.Types, sym)
					}
				} else {
					sym := typeSymbolFromDeclaration(path, decl, "")
					sym.Namespace = namespace
					sym.SourceRoot = p.Root
					sym.Version = version
					sym.Dependency = dependency
					key := namespaceTypeKey(sym.Namespace, sym.Name)
					currentPackage := p.PackagePathForFile(path)
					if previous, ok := conflictingSeenType(seenTypes[key], currentPackage); ok {
						idx.Diagnostics = append(idx.Diagnostics, duplicateDiagnostic(sym, previous.Symbol))
					} else {
						seenTypes[key] = append(seenTypes[key], seenTypeSymbol{Symbol: sym, PackagePath: currentPackage})
					}
					idx.Types = append(idx.Types, sym)
				}
			case apexast.DeclarationTrigger:
				idx.Triggers = append(idx.Triggers, TriggerSymbol{
					Name:       decl.Name,
					Namespace:  namespace,
					ObjectName: decl.ObjectName,
					Events:     decl.Events,
					File:       path,
					Dependency: dependency,
					Range:      decl.Range,
				})
			}
		}
	}
}

func namespaceTypeKey(namespace, name string) string {
	if namespace == "" {
		return strings.ToLower(name)
	}
	return strings.ToLower(namespace + "." + name)
}

type seenTypeSymbol struct {
	Symbol      TypeSymbol
	PackagePath string
}

func conflictingSeenType(previous []seenTypeSymbol, currentPackage string) (seenTypeSymbol, bool) {
	for _, candidate := range previous {
		if duplicateSymbolsConflict(currentPackage, candidate.PackagePath) {
			return candidate, true
		}
	}
	return seenTypeSymbol{}, false
}

func duplicateSymbolsConflict(currentPackage, previousPackage string) bool {
	if currentPackage != "" && previousPackage != "" && currentPackage != previousPackage {
		return false
	}
	return true
}

func UpdateApexFiles(previous Index, changedPaths, deletedPaths []string) (idx Index) {
	parser := apexast.NewParser()
	idx = Index{
		Project: previous.Project,
		Objects: previous.Objects,
	}
	deleted := pathSet(deletedPaths)
	changed := pathSet(changedPaths)
	seenTypes := make(map[string]TypeSymbol)
	for _, typ := range previous.Types {
		if deleted[cleanFilePath(typ.File)] || changed[cleanFilePath(typ.File)] {
			continue
		}
		key := strings.ToLower(typ.Name)
		seenTypes[key] = typ
		idx.Types = append(idx.Types, typ)
	}
	for _, trigger := range previous.Triggers {
		if deleted[cleanFilePath(trigger.File)] || changed[cleanFilePath(trigger.File)] {
			continue
		}
		idx.Triggers = append(idx.Triggers, trigger)
	}
	for _, path := range changedPaths {
		file, err := parser.ParseFile(path)
		if err != nil {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERTYPE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		idx.Diagnostics = append(idx.Diagnostics, file.Diagnostics...)
		if len(file.Diagnostics) > 0 {
			continue
		}
		source, sourceErr := os.ReadFile(path)
		for _, decl := range file.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
				symbols := []TypeSymbol{typeSymbolFromDeclaration(path, decl, "")}
				if sourceErr == nil {
					symbols = typeSymbolsFromDeclaration(path, decl, "", string(source))
				}
				for _, sym := range symbols {
					key := strings.ToLower(sym.Name)
					if previous, ok := seenTypes[key]; ok {
						idx.Diagnostics = append(idx.Diagnostics, duplicateDiagnostic(sym, previous))
					} else {
						seenTypes[key] = sym
					}
					idx.Types = append(idx.Types, sym)
				}
			case apexast.DeclarationTrigger:
				idx.Triggers = append(idx.Triggers, TriggerSymbol{
					Name:       decl.Name,
					ObjectName: decl.ObjectName,
					Events:     decl.Events,
					File:       path,
					Range:      decl.Range,
				})
			}
		}
	}
	sort.Slice(idx.Types, func(i, j int) bool {
		return idx.Types[i].Name < idx.Types[j].Name
	})
	sort.Slice(idx.Triggers, func(i, j int) bool {
		return idx.Triggers[i].Name < idx.Triggers[j].Name
	})
	return idx
}

func (idx Index) HasErrors() bool {
	for _, diag := range idx.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func pathSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, path := range paths {
		out[cleanFilePath(path)] = true
	}
	return out
}

func cleanFilePath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func typeSymbolsFromDeclaration(path string, decl apexast.Declaration, parent, source string) []TypeSymbol {
	sym := typeSymbolFromDeclaration(path, decl, parent)
	sym.SuperClass, sym.Interfaces = parseTypeInheritance(source, decl.Range)
	out := []TypeSymbol{sym}
	for _, member := range decl.Members {
		switch member.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			out = append(out, typeSymbolsFromDeclaration(path, member, sym.Name, source)...)
		}
	}
	return out
}

func typeSymbolFromDeclaration(path string, decl apexast.Declaration, parent string) TypeSymbol {
	name := decl.Name
	if parent != "" {
		name = parent + "." + decl.Name
	}
	sym := TypeSymbol{
		Kind:      decl.Kind,
		Name:      name,
		File:      path,
		Modifiers: decl.Modifiers,
		IsTest:    hasTestModifier(decl.Modifiers),
		Range:     decl.Range,
	}
	for _, member := range decl.Members {
		if member.Name == "" {
			continue
		}
		sym.Members = append(sym.Members, MemberSymbol{
			Kind:       member.Kind,
			Name:       member.Name,
			Type:       member.Type,
			Modifiers:  member.Modifiers,
			Parameters: member.Parameters,
			Accessors:  member.Accessors,
			IsTest:     hasTestModifier(member.Modifiers) || (member.Kind == apexast.DeclarationMethod && hasModifier(member.Modifiers, "testmethod")),
			Range:      member.Range,
		})
	}
	return sym
}

func parseTypeInheritance(source string, r diagnostic.Range) (string, []string) {
	if r.Start.Offset < 0 || r.End.Offset <= r.Start.Offset || r.End.Offset > len(source) {
		return "", nil
	}
	text := source[r.Start.Offset:r.End.Offset]
	open := strings.IndexByte(text, '{')
	if open >= 0 {
		text = text[:open]
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})
	var super string
	var interfaces []string
	for i := 0; i < len(fields); i++ {
		switch strings.ToLower(fields[i]) {
		case "extends":
			if i+1 < len(fields) {
				super = strings.TrimSpace(fields[i+1])
				i++
			}
		case "implements":
			for j := i + 1; j < len(fields); j++ {
				token := strings.TrimSpace(fields[j])
				if token == "" {
					continue
				}
				interfaces = append(interfaces, token)
			}
			return super, interfaces
		}
	}
	return super, interfaces
}

func hasTestModifier(modifiers []string) bool {
	for _, modifier := range modifiers {
		normalized := strings.ToLower(strings.TrimPrefix(modifier, "@"))
		if i := strings.IndexByte(normalized, '('); i >= 0 {
			normalized = normalized[:i]
		}
		if normalized == "istest" {
			return true
		}
	}
	return false
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, expected) {
			return true
		}
	}
	return false
}

func duplicateDiagnostic(current, previous TypeSymbol) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERTYPE001",
		Message:  fmt.Sprintf("duplicate top-level symbol %q; first seen in %s", current.Name, previous.File),
		File:     current.File,
		Range:    &current.Range,
	}
}
