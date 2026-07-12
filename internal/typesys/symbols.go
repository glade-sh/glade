package typesys

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

type Index struct {
	Project               ProjectInfo                       `json:"project"`
	Types                 []TypeSymbol                      `json:"types"`
	Triggers              []TriggerSymbol                   `json:"triggers"`
	Objects               []schema.Object                   `json:"objects"`
	CustomMetadataRecords []schema.CustomMetadataRecord     `json:"customMetadataRecords,omitempty"`
	CodeIntelSymbols      []packageartifact.CodeIntelSymbol `json:"codeIntelSymbols,omitempty"`
	CodeIntelUses         []packageartifact.CodeIntelUse    `json:"codeIntelUses,omitempty"`
	Dependencies          []DependencyInfo                  `json:"dependencies,omitempty"`
	Diagnostics           []diagnostic.Diagnostic           `json:"diagnostics,omitempty"`
}

type ProjectInfo struct {
	Root             string `json:"root"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"sourceApiVersion,omitempty"`
}

type TypeSymbol struct {
	Kind                  apexast.DeclarationKind `json:"kind"`
	Name                  string                  `json:"name"`
	File                  string                  `json:"file"`
	Namespace             string                  `json:"namespace,omitempty"`
	SourceNamespaceRemaps []namespaceremap.Rule   `json:"sourceNamespaceRemaps,omitempty"`
	SourceRoot            string                  `json:"sourceRoot,omitempty"`
	Version               string                  `json:"version,omitempty"`
	Dependency            bool                    `json:"dependency,omitempty"`
	Artifact              bool                    `json:"artifact,omitempty"`
	Modifiers             []string                `json:"modifiers,omitempty"`
	IsTest                bool                    `json:"isTest,omitempty"`
	SuperClass            string                  `json:"superClass,omitempty"`
	Interfaces            []string                `json:"interfaces,omitempty"`
	Range                 diagnostic.Range        `json:"range"`
	Members               []MemberSymbol          `json:"members,omitempty"`
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
	Name                  string                `json:"name"`
	Namespace             string                `json:"namespace,omitempty"`
	SourceNamespaceRemaps []namespaceremap.Rule `json:"sourceNamespaceRemaps,omitempty"`
	ObjectName            string                `json:"objectName"`
	Events                []string              `json:"events,omitempty"`
	File                  string                `json:"file"`
	Dependency            bool                  `json:"dependency,omitempty"`
	Range                 diagnostic.Range      `json:"range"`
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
	CaptureSource   string                  `json:"captureSource,omitempty"`
	CaptureOrgID    string                  `json:"captureOrgId,omitempty"`
	Diagnostics     []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

func Build(p project.Project, s schema.Schema) (idx Index) {
	idx, _ = BuildWithArtifacts(p, s)
	return idx
}

// BuildWithArtifacts builds the type index and returns the source artifacts
// produced during the build.
func BuildWithArtifacts(p project.Project, s schema.Schema) (idx Index, artifacts BuildArtifacts) {
	return buildWithWorkspaceSources(p, s, NewWorkspaceSources())
}

func buildWithWorkspaceSources(p project.Project, s schema.Schema, sources *WorkspaceSources) (idx Index, artifacts BuildArtifacts) {
	if sources == nil {
		sources = NewWorkspaceSources()
	}
	artifacts.Sources = sources
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
				Code:     "GLADETYPE000",
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
			if dep.Status == "loaded" && dep.ArtifactPath != "" {
				appendArtifactDependency(&idx, dep)
			}
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
		depSchema = namespaceDependencySchema(depSchema, dep.Namespace)
		beforeTypes := len(idx.Types)
		appendFlowInterviewSymbols(&idx, *dep.Project, dep.Namespace, true)
		appendProjectSymbols(&idx, parser, *dep.Project, true, dep.Namespace, dep.Version, seenTypes, sources)
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
	for _, shim := range p.PackageShims {
		appendPackageShimSymbols(&idx, parser, shim, seenTypes, sources)
	}
	appendFlowInterviewSymbols(&idx, p, "", false)
	if p.Namespace != "" {
		appendFlowInterviewSymbols(&idx, p, p.Namespace, false)
	}
	appendDataWeaveScriptResourceSymbols(&idx, p)
	appendProjectSymbols(&idx, parser, p, false, p.Namespace, "", seenTypes, sources)

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
	return idx, artifacts
}

func appendPackageShimSymbols(idx *Index, parser *apexast.Parser, shim project.PackageShim, seenTypes map[string][]seenTypeSymbol, sources *WorkspaceSources) {
	if shim.Status != "loaded" || shim.Project == nil {
		return
	}
	beforeTypes := len(idx.Types)
	appendProjectSymbols(idx, parser, *shim.Project, true, shim.Namespace, "", seenTypes, sources)
	idx.Dependencies = append(idx.Dependencies, DependencyInfo{
		Namespace:  shim.Namespace,
		SourceRoot: shim.SourceRoot,
		Status:     "shim",
		ApexTypes:  len(idx.Types) - beforeTypes,
	})
}

func appendArtifactDependency(idx *Index, dep project.ManagedPackageDependency) {
	artifact, err := packageartifact.ReadJSON(dep.ArtifactPath)
	if err != nil {
		diag := diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "dependency_load_error",
			Message:  fmt.Sprintf("managed package dependency %s artifact load failed: %v", dep.Namespace, err),
		}
		idx.Diagnostics = append(idx.Diagnostics, diag)
		idx.Dependencies = append(idx.Dependencies, DependencyInfo{
			Namespace:   dep.Namespace,
			SourceRoot:  dep.ArtifactPath,
			Version:     dep.Version,
			Status:      "load_error",
			Diagnostics: []diagnostic.Diagnostic{diag},
		})
		return
	}
	namespace := dep.Namespace
	if namespace == "" {
		namespace = artifact.Namespace
	}
	version := dep.Version
	if version == "" {
		version = artifact.Version
	}
	for _, typ := range artifact.ApexTypes {
		idx.Types = append(idx.Types, typeSymbolFromArtifact(namespace, version, typ))
	}
	idx.Objects = append(idx.Objects, artifact.Objects...)
	idx.CustomMetadataRecords = append(idx.CustomMetadataRecords, artifact.CustomMetadataRecords...)
	idx.CodeIntelSymbols = append(idx.CodeIntelSymbols, artifact.CodeIntelSymbols...)
	idx.CodeIntelUses = append(idx.CodeIntelUses, artifact.CodeIntelUses...)
	idx.Dependencies = append(idx.Dependencies, DependencyInfo{
		Namespace:       namespace,
		SourceRoot:      dep.ArtifactPath,
		Version:         version,
		Status:          dep.Status,
		ApexTypes:       len(artifact.ApexTypes),
		Objects:         len(artifact.Objects),
		Labels:          artifact.Labels,
		StaticResources: artifact.StaticResources,
		CaptureSource:   artifact.Capture.Source,
		CaptureOrgID:    artifact.Capture.OrgID,
	})
}

func typeSymbolFromArtifact(namespace, version string, typ packageartifact.ApexType) TypeSymbol {
	if typ.Namespace == "" {
		typ.Namespace = namespace
	}
	if typ.Version == "" {
		typ.Version = version
	}
	if typ.SourceRoot == "" {
		typ.SourceRoot = namespace
	}
	typ.Dependency = true
	return TypeSymbol{
		Kind:       typ.Kind,
		Name:       typ.Name,
		File:       typ.File,
		Namespace:  typ.Namespace,
		SourceRoot: typ.SourceRoot,
		Version:    typ.Version,
		Dependency: typ.Dependency,
		Artifact:   true,
		Modifiers:  append([]string(nil), typ.Modifiers...),
		IsTest:     typ.IsTest,
		SuperClass: typ.SuperClass,
		Interfaces: append([]string(nil), typ.Interfaces...),
		Range:      typ.Range,
		Members:    memberSymbolsFromArtifact(typ.Members),
	}
}

func memberSymbolsFromArtifact(members []packageartifact.ApexMember) []MemberSymbol {
	out := make([]MemberSymbol, 0, len(members))
	for _, member := range members {
		out = append(out, MemberSymbol{
			Kind:       member.Kind,
			Name:       member.Name,
			Type:       member.Type,
			Modifiers:  append([]string(nil), member.Modifiers...),
			Parameters: append([]apexast.Parameter(nil), member.Parameters...),
			Accessors:  append([]apexast.Accessor(nil), member.Accessors...),
			IsTest:     member.IsTest,
			Range:      member.Range,
		})
	}
	return out
}

func appendProjectSymbols(idx *Index, parser *apexast.Parser, p project.Project, dependency bool, namespace, version string, seenTypes map[string][]seenTypeSymbol, sources *WorkspaceSources) {
	var sourceRemaps []namespaceremap.Rule
	if dependency {
		sourceRemaps = p.NamespaceRemaps
	}
	for _, file := range projectSymbolFiles(parser, p, dependency, namespace, version, sourceRemaps, sources) {
		if file.Source != nil {
			sources.record(*file.Source)
		}
		if !dependency {
			idx.Diagnostics = append(idx.Diagnostics, file.Diagnostics...)
		}
		for _, sym := range file.Types {
			key := namespaceTypeKey(sym.Namespace, sym.Name)
			currentPackage := p.PackagePathForFile(sym.File)
			if previous, ok := conflictingSeenType(seenTypes[key], currentPackage); ok {
				if !dependency {
					idx.Diagnostics = append(idx.Diagnostics, duplicateDiagnostic(sym, previous.Symbol))
				}
			} else {
				seenTypes[key] = append(seenTypes[key], seenTypeSymbol{Symbol: sym, PackagePath: currentPackage})
			}
			idx.Types = append(idx.Types, sym)
		}
		idx.Triggers = append(idx.Triggers, file.Triggers...)
	}
}

func appendDataWeaveScriptResourceSymbols(idx *Index, p project.Project) {
	scriptNames := dataWeaveScriptResourceNames(p)
	for _, name := range scriptNames {
		idx.Types = append(idx.Types, TypeSymbol{
			Kind:       apexast.DeclarationClass,
			Name:       "DataWeaveScriptResource." + name,
			File:       dataWeaveScriptResourceFile(p, name),
			Dependency: true,
			SuperClass: "DataWeave.Script",
			Members: []MemberSymbol{
				{
					Kind:      apexast.DeclarationConstructor,
					Name:      name,
					Modifiers: []string{"public"},
				},
				{
					Kind:      apexast.DeclarationMethod,
					Name:      "execute",
					Type:      "DataWeave.Result",
					Modifiers: []string{"public", "passive-generated"},
					Parameters: []apexast.Parameter{{
						Name: "inputs",
						Type: "Map<String,Object>",
					}},
				},
			},
		})
	}
}

func appendFlowInterviewSymbols(idx *Index, p project.Project, namespace string, dependency bool) {
	for _, flow := range flowInterviewNames(p) {
		name := "Flow.Interview." + flow
		if namespace != "" {
			name = "Flow.Interview." + namespace + "." + flow
		}
		idx.Types = append(idx.Types, TypeSymbol{
			Kind:       apexast.DeclarationClass,
			Name:       name,
			File:       flowInterviewFile(p, flow),
			Dependency: dependency,
			SourceRoot: p.Root,
			SuperClass: "Flow.Interview",
			Members: []MemberSymbol{{
				Kind:      apexast.DeclarationConstructor,
				Name:      flow,
				Modifiers: []string{"public"},
				Parameters: []apexast.Parameter{{
					Name: "inputVariables",
					Type: "Map<String,Object>",
				}},
			}},
		})
	}
}

func flowInterviewNames(p project.Project) []string {
	seen := make(map[string]bool)
	for _, path := range p.FlowFiles {
		name := flowInterviewName(path)
		if name != "" {
			seen[strings.ToLower(name)] = true
		}
	}
	names := make([]string, 0, len(seen))
	for _, path := range p.FlowFiles {
		name := flowInterviewName(path)
		key := strings.ToLower(name)
		if name == "" || !seen[key] {
			continue
		}
		names = append(names, name)
		delete(seen, key)
	}
	sort.Strings(names)
	return names
}

func flowInterviewName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, "-meta.xml")
	base = strings.TrimSuffix(base, ".flow")
	if base == "" || base == filepath.Base(path) {
		return ""
	}
	return base
}

func flowInterviewFile(p project.Project, name string) string {
	for _, path := range p.FlowFiles {
		if strings.EqualFold(flowInterviewName(path), name) {
			return path
		}
	}
	return "<flow>"
}

func dataWeaveScriptResourceNames(p project.Project) []string {
	seen := make(map[string]bool)
	for _, path := range append(append([]string{}, p.DataWeaveFiles...), p.DataWeaveMetas...) {
		name := dataWeaveScriptResourceName(path)
		if name != "" {
			seen[strings.ToLower(name)] = true
		}
	}
	names := make([]string, 0, len(seen))
	for _, path := range append(append([]string{}, p.DataWeaveFiles...), p.DataWeaveMetas...) {
		name := dataWeaveScriptResourceName(path)
		key := strings.ToLower(name)
		if name == "" || !seen[key] {
			continue
		}
		names = append(names, name)
		delete(seen, key)
	}
	sort.Strings(names)
	return names
}

func dataWeaveScriptResourceName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, "-meta.xml")
	base = strings.TrimSuffix(base, ".dwl")
	if base == "" || base == filepath.Base(path) {
		return ""
	}
	return base
}

func dataWeaveScriptResourceFile(p project.Project, name string) string {
	for _, path := range p.DataWeaveFiles {
		if strings.EqualFold(dataWeaveScriptResourceName(path), name) {
			return path
		}
	}
	for _, path := range p.DataWeaveMetas {
		if strings.EqualFold(dataWeaveScriptResourceName(path), name) {
			return path
		}
	}
	return "<dataweave>"
}

type projectSymbolFile struct {
	Diagnostics []diagnostic.Diagnostic
	Types       []TypeSymbol
	Triggers    []TriggerSymbol
	Source      *WorkspaceSource
}

func projectSymbolFiles(parser *apexast.Parser, p project.Project, dependency bool, namespace, version string, sourceRemaps []namespaceremap.Rule, sources *WorkspaceSources) []projectSymbolFile {
	if len(p.ApexFiles) == 0 {
		return nil
	}
	if len(p.ApexFiles) == 1 {
		return []projectSymbolFile{projectSymbolFileFromPath(parser, p.ApexFiles[0], p.Root, dependency, namespace, version, sourceRemaps, sources)}
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > len(p.ApexFiles) {
		workers = len(p.ApexFiles)
	}
	if workers > 8 {
		workers = 8
	}

	type job struct {
		Index int
		Path  string
	}
	type result struct {
		Index int
		File  projectSymbolFile
	}
	jobs := make(chan job)
	results := make(chan result, len(p.ApexFiles))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localParser := apexast.NewParser()
			for job := range jobs {
				results <- result{
					Index: job.Index,
					File:  projectSymbolFileFromPath(localParser, job.Path, p.Root, dependency, namespace, version, sourceRemaps, sources),
				}
			}
		}()
	}
	go func() {
		for i, path := range p.ApexFiles {
			jobs <- job{Index: i, Path: path}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	out := make([]projectSymbolFile, len(p.ApexFiles))
	for result := range results {
		out[result.Index] = result.File
	}
	return out
}

func projectSymbolFileFromPath(parser *apexast.Parser, path, root string, dependency bool, namespace, version string, sourceRemaps []namespaceremap.Rule, sources *WorkspaceSources) projectSymbolFile {
	var out projectSymbolFile
	source, err := sources.load(SourceMetadata{
		RequestedPath:   path,
		Root:            root,
		Namespace:       namespace,
		Version:         version,
		Dependency:      dependency,
		NamespaceRemaps: sourceRemaps,
	})
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADETYPE000",
			Message:  err.Error(),
			File:     path,
		})
		return out
	}
	out.Source = &source
	normalized := source.NormalizedString()
	file := parser.ParseSource(path, normalized)
	out.Diagnostics = append(out.Diagnostics, file.Diagnostics...)
	if len(file.Diagnostics) > 0 {
		return out
	}
	for _, decl := range file.Declarations {
		switch decl.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			for _, sym := range typeSymbolsFromDeclaration(path, decl, "", false, normalized) {
				sym.Namespace = namespace
				sym.SourceNamespaceRemaps = append([]namespaceremap.Rule(nil), sourceRemaps...)
				sym.SourceRoot = root
				sym.Version = version
				sym.Dependency = dependency
				out.Types = append(out.Types, sym)
			}
		case apexast.DeclarationTrigger:
			out.Triggers = append(out.Triggers, TriggerSymbol{
				Name:                  decl.Name,
				Namespace:             namespace,
				SourceNamespaceRemaps: append([]namespaceremap.Rule(nil), sourceRemaps...),
				ObjectName:            decl.ObjectName,
				Events:                decl.Events,
				File:                  path,
				Dependency:            dependency,
				Range:                 decl.Range,
			})
		}
	}
	return out
}

func namespaceTypeKey(namespace, name string) string {
	if namespace == "" {
		return strings.ToLower(name)
	}
	return strings.ToLower(namespace + "." + name)
}

func namespaceDependencySchema(in schema.Schema, namespace string) schema.Schema {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return in
	}
	out := in
	out.Objects = make([]schema.Object, len(in.Objects))
	for i, object := range in.Objects {
		object.Name = namespaceAPIName(namespace, object.Name)
		for j, field := range object.Fields {
			field.Name = namespaceAPIName(namespace, field.Name)
			for k, referenceTo := range field.ReferenceTo {
				field.ReferenceTo[k] = namespaceAPIName(namespace, referenceTo)
			}
			field.RelationshipName = namespaceAPIName(namespace, field.RelationshipName)
			field.ChildRelationshipName = namespaceAPIName(namespace, field.ChildRelationshipName)
			field.SummarizedField = namespaceAPIName(namespace, field.SummarizedField)
			field.SummaryForeignKey = namespaceAPIName(namespace, field.SummaryForeignKey)
			for k, filter := range field.SummaryFilterItems {
				filter.Field = namespaceAPIName(namespace, filter.Field)
				field.SummaryFilterItems[k] = filter
			}
			object.Fields[j] = field
		}
		for j, rule := range object.ValidationRules {
			rule.ErrorDisplayField = namespaceAPIName(namespace, rule.ErrorDisplayField)
			object.ValidationRules[j] = rule
		}
		out.Objects[i] = object
	}
	out.CustomMetadataRecords = make([]schema.CustomMetadataRecord, len(in.CustomMetadataRecords))
	for i, record := range in.CustomMetadataRecords {
		record.ObjectName = namespaceAPIName(namespace, record.ObjectName)
		record.FullName = namespaceMetadataFullName(namespace, record.FullName)
		for j, value := range record.Values {
			value.Field = namespaceAPIName(namespace, value.Field)
			record.Values[j] = value
		}
		out.CustomMetadataRecords[i] = record
	}
	return out
}

func namespaceMetadataFullName(namespace, fullName string) string {
	objectName, developerName, ok := strings.Cut(fullName, ".")
	if !ok {
		return namespaceAPIName(namespace, fullName)
	}
	return namespaceAPIName(namespace, objectName) + "." + developerName
}

func namespaceAPIName(namespace, name string) string {
	if namespace == "" || name == "" || !customAPIName(name) || hasNamespaceToken(name) {
		return name
	}
	return namespace + "__" + name
}

func customAPIName(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{"__c", "__r", "__e", "__mdt", "__b"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func hasNamespaceToken(name string) bool {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	return first > 0 && first < last
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
		data, err := os.ReadFile(path)
		if err != nil {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADETYPE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		source := project.NormalizeApexNamespaceTokens(string(data), previous.Project.Namespace)
		file := parser.ParseSource(path, source)
		idx.Diagnostics = append(idx.Diagnostics, file.Diagnostics...)
		if len(file.Diagnostics) > 0 {
			continue
		}
		for _, decl := range file.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
				for _, sym := range typeSymbolsFromDeclaration(path, decl, "", false, source) {
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

func typeSymbolsFromDeclaration(path string, decl apexast.Declaration, parent string, parentIsTest bool, source string) []TypeSymbol {
	sym := typeSymbolFromDeclaration(path, decl, parent, parentIsTest)
	sym.SuperClass, sym.Interfaces = parseTypeInheritance(source, decl.Range)
	out := []TypeSymbol{sym}
	for _, member := range decl.Members {
		switch member.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			out = append(out, typeSymbolsFromDeclaration(path, member, sym.Name, sym.IsTest, source)...)
		}
	}
	return out
}

func typeSymbolFromDeclaration(path string, decl apexast.Declaration, parent string, parentIsTest bool) TypeSymbol {
	name := decl.Name
	if parent != "" {
		name = parent + "." + decl.Name
	}
	sym := TypeSymbol{
		Kind:      decl.Kind,
		Name:      name,
		File:      path,
		Modifiers: decl.Modifiers,
		IsTest:    parentIsTest || hasTestModifier(decl.Modifiers),
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
	text := stripTypeInheritanceComments(source[r.Start.Offset:r.End.Offset])
	open := strings.IndexByte(text, '{')
	if open >= 0 {
		text = text[:open]
	}
	fields := splitTypeInheritanceFields(text)
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

func stripTypeInheritanceComments(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '/' && i+1 < len(text) {
			switch text[i+1] {
			case '/':
				for i < len(text) && text[i] != '\n' {
					i++
				}
				if i < len(text) {
					out.WriteByte(text[i])
				}
				continue
			case '*':
				i += 2
				for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
					if text[i] == '\n' {
						out.WriteByte('\n')
					} else {
						out.WriteByte(' ')
					}
					i++
				}
				if i+1 < len(text) {
					i++
				}
				continue
			}
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

func splitTypeInheritanceFields(text string) []string {
	var fields []string
	start := -1
	depth := 0
	for i, r := range text {
		switch r {
		case '<':
			if start < 0 {
				start = i
			}
			depth++
		case '>':
			if start < 0 {
				start = i
			}
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\n', '\r', ',':
			if depth == 0 {
				if start >= 0 {
					fields = append(fields, strings.TrimSpace(text[start:i]))
					start = -1
				}
				continue
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		fields = append(fields, strings.TrimSpace(text[start:]))
	}
	return fields
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
		Severity: diagnostic.Warning,
		Code:     "GLADETYPE001",
		Message:  fmt.Sprintf("duplicate top-level symbol %q; first seen in %s", current.Name, previous.File),
		File:     current.File,
		Range:    &current.Range,
	}
}
