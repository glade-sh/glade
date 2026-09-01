package typesys

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// Index is an immutable snapshot. Nested payloads may be structurally shared
// between snapshots; callers must not mutate an Index after publication.
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
	projectIdentity       string
	sourceDigests         *SourceDigestSet
	apexMetadataInputs    map[sourceOccurrenceKey]ApexMetadataInput
}

type ProjectInfo struct {
	Root             string `json:"root"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"sourceApiVersion,omitempty"`
}

// ApexMetadataForType returns the companion metadata identity retained in this
// index generation for a logical Apex occurrence.
func (i Index) ApexMetadataForType(typ TypeSymbol) (ApexMetadataInput, bool) {
	input, ok := i.apexMetadataInputs[sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace, Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps})]
	return input, ok
}

func (i Index) ApexMetadataForTrigger(trigger TriggerSymbol) (ApexMetadataInput, bool) {
	input, ok := i.apexMetadataInputs[sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: trigger.File, Root: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, NamespaceRemaps: trigger.SourceNamespaceRemaps})]
	return input, ok
}

type TypeSymbol struct {
	Kind                  apexast.DeclarationKind `json:"kind"`
	Name                  string                  `json:"name"`
	LocalName             string                  `json:"localName,omitempty"`
	OwnerName             string                  `json:"ownerName,omitempty"`
	NestingDepth          int                     `json:"nestingDepth,omitempty"`
	File                  string                  `json:"file"`
	Namespace             string                  `json:"namespace,omitempty"`
	SourceNamespaceRemaps []namespaceremap.Rule   `json:"sourceNamespaceRemaps,omitempty"`
	SourceRoot            string                  `json:"sourceRoot,omitempty"`
	Version               string                  `json:"version,omitempty"`
	EffectiveAPIVersion   string                  `json:"effectiveApiVersion,omitempty"`
	// SourceBacked marks a symbol parsed from an Apex source occurrence held by
	// BuildArtifacts. Generated metadata and serialized artifact types may have
	// a File for diagnostics without having a readable Apex source snapshot.
	SourceBacked   bool                 `json:"sourceBacked,omitempty"`
	Dependency     bool                 `json:"dependency,omitempty"`
	Artifact       bool                 `json:"artifact,omitempty"`
	Modifiers      []string             `json:"modifiers,omitempty"`
	Annotations    []apexast.Annotation `json:"annotations,omitempty"`
	TypeParameters []string             `json:"typeParameters,omitempty"`
	IsTest         bool                 `json:"isTest,omitempty"`
	// ConstructorsAuthoritative is internal standard-platform metadata. It
	// closes an intentionally empty or narrowed constructor set without
	// exposing a synthetic Apex modifier through symbol JSON.
	ConstructorsAuthoritative bool             `json:"-"`
	EnumHashBase              *int64           `json:"-"` // measured platform enum family identity seed
	SuperClass                string           `json:"superClass,omitempty"`
	Interfaces                []string         `json:"interfaces,omitempty"`
	Range                     diagnostic.Range `json:"range"`
	Members                   []MemberSymbol   `json:"members,omitempty"`
}

// HasSourceSnapshot reports whether this symbol requires an Apex source
// occurrence in BuildArtifacts. SourceBacked is authoritative for newly built
// indexes. The suffix fallback preserves safe incremental behavior for index
// snapshots serialized before SourceBacked was introduced.
func (s TypeSymbol) HasSourceSnapshot() bool {
	return s.SourceBacked || (!s.Artifact && strings.EqualFold(filepath.Ext(s.File), ".cls"))
}

type MemberSymbol struct {
	Kind        apexast.DeclarationKind `json:"kind"`
	Name        string                  `json:"name"`
	Type        string                  `json:"type,omitempty"`
	Modifiers   []string                `json:"modifiers,omitempty"`
	Annotations []apexast.Annotation    `json:"annotations,omitempty"`
	Parameters  []apexast.Parameter     `json:"parameters,omitempty"`
	Accessors   []apexast.Accessor      `json:"accessors,omitempty"`
	HasBody     bool                    `json:"hasBody,omitempty"`
	BodyRange   *diagnostic.Range       `json:"bodyRange,omitempty"`
	IsTest      bool                    `json:"isTest,omitempty"`
	Range       diagnostic.Range        `json:"range"`
}

type TriggerSymbol struct {
	Name                  string                `json:"name"`
	Namespace             string                `json:"namespace,omitempty"`
	SourceNamespaceRemaps []namespaceremap.Rule `json:"sourceNamespaceRemaps,omitempty"`
	SourceRoot            string                `json:"sourceRoot,omitempty"`
	Version               string                `json:"version,omitempty"`
	EffectiveAPIVersion   string                `json:"effectiveApiVersion,omitempty"`
	// SourceBacked has the same source-arena contract as TypeSymbol.
	SourceBacked bool              `json:"sourceBacked,omitempty"`
	ObjectName   string            `json:"objectName"`
	Events       []string          `json:"events,omitempty"`
	File         string            `json:"file"`
	Dependency   bool              `json:"dependency,omitempty"`
	BodyRange    *diagnostic.Range `json:"bodyRange,omitempty"`
	Range        diagnostic.Range  `json:"range"`
}

// HasSourceSnapshot reports whether this trigger requires an Apex source
// occurrence in BuildArtifacts. The suffix fallback supports older index
// snapshots that predate SourceBacked.
func (s TriggerSymbol) HasSourceSnapshot() bool {
	return s.SourceBacked || strings.EqualFold(filepath.Ext(s.File), ".trigger")
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
		projectIdentity:       incrementalProjectIdentity(p),
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
	artifacts.SourceDigests = sources.sourceDigestSet()
	artifacts.ApexMetadataInputs = capturedApexMetadataInputs(idx, sources)
	idx.sourceDigests = artifacts.SourceDigests
	idx.apexMetadataInputs = sources.apexMetadataInputSet()

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

func capturedApexMetadataInputs(idx Index, sources *WorkspaceSources) map[string]ApexMetadataInput {
	inputs := make(map[string]ApexMetadataInput)
	captureType := func(typ TypeSymbol) {
		file := typ.File
		if file == "" {
			return
		}
		if _, seen := inputs[file]; seen {
			return
		}
		if input, ok := sources.apexMetadataForMetadata(SourceMetadata{
			RequestedPath: file, Root: typ.SourceRoot, Namespace: typ.Namespace,
			Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps,
		}); ok {
			inputs[file] = input
		}
	}
	for _, typ := range idx.Types {
		if typ.HasSourceSnapshot() {
			captureType(typ)
		}
	}
	for _, trigger := range idx.Triggers {
		if trigger.HasSourceSnapshot() {
			captureType(TypeSymbol{File: trigger.File, SourceRoot: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, SourceNamespaceRemaps: trigger.SourceNamespaceRemaps})
		}
	}
	return inputs
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
	if issues := packageartifact.Validate(artifact); len(issues) > 0 {
		diag := diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "dependency_load_error",
			Message:  fmt.Sprintf("managed package dependency %s artifact invalid: %s", dep.Namespace, strings.Join(issues, "; ")),
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
		idx.Types = append(idx.Types, typeSymbolFromArtifact(namespace, version, artifact.SourceAPIVersion, typ))
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

func typeSymbolFromArtifact(namespace, version, sourceAPIVersion string, typ packageartifact.ApexType) TypeSymbol {
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
		Kind:                typ.Kind,
		Name:                typ.Name,
		File:                typ.File,
		Namespace:           typ.Namespace,
		SourceRoot:          typ.SourceRoot,
		Version:             typ.Version,
		EffectiveAPIVersion: sourceAPIVersion,
		Dependency:          typ.Dependency,
		Artifact:            true,
		Modifiers:           append([]string(nil), typ.Modifiers...),
		IsTest:              typ.IsTest,
		SuperClass:          typ.SuperClass,
		Interfaces:          append([]string(nil), typ.Interfaces...),
		Range:               typ.Range,
		Members:             memberSymbolsFromArtifact(typ.Members),
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
			sources.recordApexMetadata(*file.Source, file.Metadata)
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
	Metadata    ApexMetadataInput
}

var (
	apexMetadataCaptureHookMu sync.RWMutex
	apexMetadataCaptureHook   func(string)
)

func setApexMetadataCaptureHookForTesting(hook func(string)) func() {
	apexMetadataCaptureHookMu.Lock()
	previous := apexMetadataCaptureHook
	apexMetadataCaptureHook = hook
	apexMetadataCaptureHookMu.Unlock()
	return func() {
		apexMetadataCaptureHookMu.Lock()
		apexMetadataCaptureHook = previous
		apexMetadataCaptureHookMu.Unlock()
	}
}

func captureApexMetadataInput(path, fallback string) (ApexMetadataInput, string) {
	data, err := os.ReadFile(path + "-meta.xml") // #nosec G304 -- fixed metadata companion for an indexed Apex source.
	input := ApexMetadataInput{}
	effective := fallback
	if err == nil {
		input = ApexMetadataInput{Present: true, Digest: sha256.Sum256(data)}
		effective = project.EffectiveSourceAPIVersionFromMetadata(data, fallback)
	}
	apexMetadataCaptureHookMu.RLock()
	hook := apexMetadataCaptureHook
	apexMetadataCaptureHookMu.RUnlock()
	if hook != nil {
		hook(path)
	}
	return input, effective
}

func sourceStillMatches(path string, expected [sha256.Size]byte) bool {
	data, err := os.ReadFile(path) // #nosec G304 -- source just captured for this index occurrence.
	return err == nil && sha256.Sum256(data) == expected
}

func metadataStillMatches(path string, expected ApexMetadataInput) bool {
	data, err := os.ReadFile(path + "-meta.xml") // #nosec G304 -- fixed metadata companion for the indexed Apex source.
	if err != nil {
		return !expected.Present && os.IsNotExist(err)
	}
	return expected.Present && sha256.Sum256(data) == expected.Digest
}

func projectSymbolFiles(parser *apexast.Parser, p project.Project, dependency bool, namespace, version string, sourceRemaps []namespaceremap.Rule, sources *WorkspaceSources) []projectSymbolFile {
	if len(p.ApexFiles) == 0 {
		return nil
	}
	if len(p.ApexFiles) == 1 {
		return []projectSymbolFile{projectSymbolFileFromPath(parser, p.ApexFiles[0], p.Root, p.SourceAPIVersion, dependency, namespace, version, sourceRemaps, sources)}
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
					File:  projectSymbolFileFromPath(localParser, job.Path, p.Root, p.SourceAPIVersion, dependency, namespace, version, sourceRemaps, sources),
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

func projectSymbolFileFromPath(parser *apexast.Parser, path, root, fallbackAPIVersion string, dependency bool, namespace, version string, sourceRemaps []namespaceremap.Rule, sources *WorkspaceSources) projectSymbolFile {
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
	var effectiveAPIVersion string
	out.Metadata, effectiveAPIVersion = captureApexMetadataInput(path, fallbackAPIVersion)
	if !sourceStillMatches(path, source.Digest()) || !metadataStillMatches(path, out.Metadata) {
		out.Diagnostics = append(out.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADETYPE001", Message: "Apex source changed while capturing companion metadata", File: path})
		return out
	}
	normalized := source.NormalizedString()
	file := parser.ParseSource(path, normalized)
	out.Diagnostics = append(out.Diagnostics, file.Diagnostics...)
	if hasBlockingParserDiagnostic(file.Diagnostics) {
		return out
	}
	for _, decl := range file.Declarations {
		switch decl.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			for _, sym := range typeSymbolsFromDeclaration(path, decl, "", 0, false) {
				sym.Namespace = namespace
				sym.SourceNamespaceRemaps = append([]namespaceremap.Rule(nil), sourceRemaps...)
				sym.SourceRoot = root
				sym.Version = version
				sym.EffectiveAPIVersion = effectiveAPIVersion
				sym.SourceBacked = true
				sym.Dependency = dependency
				out.Types = append(out.Types, sym)
			}
		case apexast.DeclarationTrigger:
			out.Triggers = append(out.Triggers, TriggerSymbol{
				Name:                  decl.Name,
				Namespace:             namespace,
				SourceNamespaceRemaps: append([]namespaceremap.Rule(nil), sourceRemaps...),
				SourceRoot:            root,
				Version:               version,
				EffectiveAPIVersion:   effectiveAPIVersion,
				SourceBacked:          true,
				ObjectName:            decl.ObjectName,
				Events:                decl.Events,
				File:                  path,
				Dependency:            dependency,
				BodyRange:             decl.BodyRange,
				Range:                 decl.Range,
			})
		}
	}
	return out
}

func hasBlockingParserDiagnostic(diagnostics []diagnostic.Diagnostic) bool {
	for _, diag := range diagnostics {
		switch diag.Code {
		case "APEXPARSE002", "APEXPARSE003":
			continue
		default:
			return true
		}
	}
	return false
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

func UpdateApexFiles(previous Index, changedPaths, deletedPaths []string) Index {
	idx, err := UpdateApexFilesChecked(previous, changedPaths, deletedPaths)
	if err != nil {
		return incrementalFallbackFailure(previous, err)
	}
	return idx
}

// UpdateApexFilesChecked updates Apex sources and reports an error when the
// authoritative fallback index cannot be loaded.
func UpdateApexFilesChecked(previous Index, changedPaths, deletedPaths []string) (Index, error) {
	return updateApexFilesCheckedWithIdentityOps(previous, changedPaths, deletedPaths, incrementalFileIdentityOps{})
}

// TryUpdateApexFilesChecked applies only an exact incremental update. The
// boolean is false when the caller must reload one authoritative project and
// rebuild from it; no fallback index is returned in that case.
func TryUpdateApexFilesChecked(previous Index, changedPaths, deletedPaths []string) (Index, bool, error) {
	return tryUpdateApexFilesCheckedWithIdentityOps(previous, changedPaths, deletedPaths, incrementalFileIdentityOps{})
}

// TryUpdateApexFilesCheckedWithLoadedProject applies only an exact
// incremental update while reusing an authoritative project already loaded by
// the caller. The boolean is false when the caller must rebuild the complete
// index from p.
func TryUpdateApexFilesCheckedWithLoadedProject(previous Index, changedPaths, deletedPaths []string, p project.Project) (Index, bool, error) {
	identityOps := incrementalIdentityOpsWithDefaults(incrementalFileIdentityOps{})
	if idx, ok := updateApexFilesIncrementalWithLoadedProject(previous, changedPaths, deletedPaths, identityOps, p); ok {
		return idx, true, nil
	}
	return Index{}, false, nil
}

// MatchesProjectIdentity reports whether p has the same configuration and
// dependency identity as the authoritative project used to build idx.
func MatchesProjectIdentity(idx Index, p project.Project) bool {
	return idx.projectIdentity != "" && idx.projectIdentity == incrementalProjectIdentity(p) &&
		p.Root == idx.Project.Root && p.Namespace == idx.Project.Namespace &&
		p.SourceAPIVersion == idx.Project.SourceAPIVersion
}

// SameProjectIdentity reports whether two immutable indexes were built from
// the same project configuration and dependency identity.
func SameProjectIdentity(left, right Index) bool {
	return left.projectIdentity != "" && left.projectIdentity == right.projectIdentity
}

// ProjectIdentityDigest returns the immutable private project and dependency
// identity captured by the build that produced idx. The digest deliberately
// does not expose the private identity payload to callers.
func ProjectIdentityDigest(idx Index) ([sha256.Size]byte, bool) {
	if idx.projectIdentity == "" {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256([]byte(idx.projectIdentity)), true
}

// SourceDigest returns the raw source digest retained by the build that
// produced idx. It never reads the filesystem.
func (idx Index) SourceDigest(path string) ([32]byte, bool) {
	return idx.sourceDigests.Digest(path)
}

// RequiresAuthoritativeApexRebuild reports whether the change shape cannot use
// the exact incremental updater without first loading project identity.
func RequiresAuthoritativeApexRebuild(previous Index, changedPaths, deletedPaths []string) bool {
	return !incrementalUpdateShapeSupported(previous, changedPaths, deletedPaths)
}

func updateApexFilesCheckedWithIdentityOps(previous Index, changedPaths, deletedPaths []string, identityOps incrementalFileIdentityOps) (Index, error) {
	idx, exact, p, err := tryUpdateApexFilesCheckedWithLoadedProject(previous, changedPaths, deletedPaths, identityOps)
	if err != nil {
		return Index{}, err
	}
	if exact {
		return idx, nil
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return Index{}, err
	}
	return Build(p, s), nil
}

func tryUpdateApexFilesCheckedWithIdentityOps(previous Index, changedPaths, deletedPaths []string, identityOps incrementalFileIdentityOps) (Index, bool, error) {
	idx, exact, _, err := tryUpdateApexFilesCheckedWithLoadedProject(previous, changedPaths, deletedPaths, identityOps)
	return idx, exact, err
}

func tryUpdateApexFilesCheckedWithLoadedProject(previous Index, changedPaths, deletedPaths []string, identityOps incrementalFileIdentityOps) (Index, bool, project.Project, error) {
	identityOps = incrementalIdentityOpsWithDefaults(identityOps)
	p, err := identityOps.loadProject(previous.Project.Root)
	if err != nil {
		return Index{}, false, project.Project{}, err
	}
	if idx, ok := updateApexFilesIncrementalWithLoadedProject(previous, changedPaths, deletedPaths, identityOps, p); ok {
		return idx, true, p, nil
	}
	return Index{}, false, p, nil
}

func incrementalFallbackFailure(previous Index, err error) Index {
	idx := previous
	idx.Diagnostics = append(append([]diagnostic.Diagnostic(nil), previous.Diagnostics...), diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADETYPE000",
		Message:  fmt.Sprintf("incremental fallback build failed: %v", err),
	})
	return idx
}

type incrementalFileIdentityOps struct {
	stat        func(string) (os.FileInfo, error)
	lstat       func(string) (os.FileInfo, error)
	sameFile    func(os.FileInfo, os.FileInfo) bool
	loadProject func(string) (project.Project, error)
}

func incrementalIdentityOpsWithDefaults(identityOps incrementalFileIdentityOps) incrementalFileIdentityOps {
	if identityOps.stat == nil {
		identityOps.stat = os.Stat
	}
	if identityOps.lstat == nil {
		identityOps.lstat = os.Lstat
	}
	if identityOps.sameFile == nil {
		identityOps.sameFile = os.SameFile
	}
	if identityOps.loadProject == nil {
		identityOps.loadProject = project.Load
	}
	return identityOps
}

type incrementalSourceMetadata struct {
	namespace       string
	namespaceRemaps []namespaceremap.Rule
	root            string
	version         string
	apiVersion      string
	dependency      bool
}

type incrementalSourceOwner struct {
	project         project.Project
	metadata        incrementalSourceMetadata
	dependencyIndex int
	supported       bool
}

type incrementalResolvedSource struct {
	owner       incrementalSourceOwner
	path        string
	packagePath string
}

type incrementalProjectConfigIdentity struct {
	Root               string
	Namespace          string
	SourceAPIVersion   string
	PackageDirectories []project.PackageDirectory
	NamespaceRemaps    []namespaceremap.Rule
}

type incrementalManagedDependencyIdentity struct {
	Namespace    string
	SourceRoot   string
	ArtifactPath string
	Version      string
	Status       string
	Project      *incrementalProjectConfigIdentity
}

type incrementalPackageShimIdentity struct {
	Namespace  string
	SourceRoot string
	Status     string
	Project    *incrementalProjectConfigIdentity
}

type incrementalProjectIdentityLedger struct {
	Project               incrementalProjectConfigIdentity
	ManagedDependencies   []incrementalManagedDependencyIdentity
	PackageShims          []incrementalPackageShimIdentity
	DependencyDiagnostics []project.DependencyDiagnostic
}

func incrementalProjectConfigForIdentity(p project.Project) incrementalProjectConfigIdentity {
	return incrementalProjectConfigIdentity{
		Root:               p.Root,
		Namespace:          p.Namespace,
		SourceAPIVersion:   p.SourceAPIVersion,
		PackageDirectories: p.PackageDirectories,
		NamespaceRemaps:    p.NamespaceRemaps,
	}
}

func incrementalProjectIdentity(p project.Project) string {
	ledger := incrementalProjectIdentityLedger{
		Project:               incrementalProjectConfigForIdentity(p),
		DependencyDiagnostics: p.DependencyDiagnostics,
	}
	for _, dep := range p.ManagedPackageDependencies {
		identity := incrementalManagedDependencyIdentity{
			Namespace:    dep.Namespace,
			SourceRoot:   dep.SourceRoot,
			ArtifactPath: dep.ArtifactPath,
			Version:      dep.Version,
			Status:       dep.Status,
		}
		if dep.Project != nil {
			projectIdentity := incrementalProjectConfigForIdentity(*dep.Project)
			identity.Project = &projectIdentity
		}
		ledger.ManagedDependencies = append(ledger.ManagedDependencies, identity)
	}
	for _, shim := range p.PackageShims {
		identity := incrementalPackageShimIdentity{
			Namespace:  shim.Namespace,
			SourceRoot: shim.SourceRoot,
			Status:     shim.Status,
		}
		if shim.Project != nil {
			projectIdentity := incrementalProjectConfigForIdentity(*shim.Project)
			identity.Project = &projectIdentity
		}
		ledger.PackageShims = append(ledger.PackageShims, identity)
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		return ""
	}
	return string(data)
}

func cloneIncrementalNamespaceRemaps(in []namespaceremap.Rule) []namespaceremap.Rule {
	if in == nil {
		return nil
	}
	out := make([]namespaceremap.Rule, len(in))
	copy(out, in)
	return out
}

func sameIncrementalSourceMetadata(left, right incrementalSourceMetadata) bool {
	if left.namespace != right.namespace || left.root != right.root || left.version != right.version || left.apiVersion != right.apiVersion || left.dependency != right.dependency || len(left.namespaceRemaps) != len(right.namespaceRemaps) {
		return false
	}
	for i := range left.namespaceRemaps {
		if left.namespaceRemaps[i] != right.namespaceRemaps[i] {
			return false
		}
	}
	return true
}

func incrementalSourceOwners(previous Index, p project.Project) ([]incrementalSourceOwner, bool) {
	if previous.projectIdentity == "" || previous.projectIdentity != incrementalProjectIdentity(p) || p.Root != previous.Project.Root || p.Namespace != previous.Project.Namespace || p.SourceAPIVersion != previous.Project.SourceAPIVersion {
		return nil, false
	}
	owners := []incrementalSourceOwner{{
		project: p,
		metadata: incrementalSourceMetadata{
			namespace:  p.Namespace,
			root:       p.Root,
			apiVersion: p.SourceAPIVersion,
		},
		dependencyIndex: -1,
		supported:       true,
	}}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		dependencyIndex := -1
		for i, info := range previous.Dependencies {
			if info.Namespace != dep.Namespace || info.SourceRoot != dep.SourceRoot || info.Version != dep.Version || info.Status != dep.Status {
				continue
			}
			if dependencyIndex != -1 {
				return nil, false
			}
			dependencyIndex = i
		}
		owners = append(owners, incrementalSourceOwner{
			project: *dep.Project,
			metadata: incrementalSourceMetadata{
				namespace:       dep.Namespace,
				namespaceRemaps: cloneIncrementalNamespaceRemaps(dep.Project.NamespaceRemaps),
				root:            dep.Project.Root,
				version:         dep.Version,
				apiVersion:      dep.Project.SourceAPIVersion,
				dependency:      true,
			},
			dependencyIndex: dependencyIndex,
			supported:       dependencyIndex != -1,
		})
	}
	for _, shim := range p.PackageShims {
		if shim.Status != "loaded" || shim.Project == nil {
			continue
		}
		owners = append(owners, incrementalSourceOwner{
			project: *shim.Project,
			metadata: incrementalSourceMetadata{
				namespace:  shim.Namespace,
				root:       shim.Project.Root,
				apiVersion: shim.Project.SourceAPIVersion,
				dependency: true,
			},
			dependencyIndex: -1,
			supported:       false,
		})
	}
	return owners, true
}

func resolveIncrementalSource(owners []incrementalSourceOwner, path string, requireListed bool) (incrementalResolvedSource, bool) {
	key := cleanFilePath(path)
	var matches []incrementalResolvedSource
	for _, owner := range owners {
		listedPath := ""
		for _, apexPath := range owner.project.ApexFiles {
			if cleanFilePath(apexPath) == key {
				if listedPath != "" && listedPath != apexPath {
					return incrementalResolvedSource{}, false
				}
				listedPath = apexPath
			}
		}
		if requireListed && listedPath == "" {
			continue
		}
		if !requireListed && listedPath != "" {
			return incrementalResolvedSource{}, false
		}
		packagePath := ""
		deepestRoot := ""
		deepestMatches := 0
		for _, pkg := range owner.project.PackageDirectories {
			if pkg.Path == "" {
				continue
			}
			root := cleanFilePath(filepath.Join(owner.project.Root, filepath.FromSlash(pkg.Path)))
			if key != root && !strings.HasPrefix(key, root+string(os.PathSeparator)) {
				continue
			}
			switch {
			case len(root) > len(deepestRoot):
				deepestRoot = root
				deepestMatches = 1
				packagePath = filepath.ToSlash(filepath.Clean(pkg.Path))
			case len(root) == len(deepestRoot):
				deepestMatches++
			}
		}
		if deepestMatches == 0 {
			continue
		}
		if deepestMatches != 1 {
			return incrementalResolvedSource{}, false
		}
		if listedPath == "" {
			listedPath = path
		}
		matches = append(matches, incrementalResolvedSource{owner: owner, path: listedPath, packagePath: packagePath})
	}
	if len(matches) != 1 || !matches[0].owner.supported {
		return incrementalResolvedSource{}, false
	}
	return matches[0], true
}

func updateApexFilesIncremental(previous Index, changedPaths, deletedPaths []string) (Index, bool) {
	return updateApexFilesIncrementalWithIdentityOps(previous, changedPaths, deletedPaths, incrementalFileIdentityOps{})
}

func updateApexFilesIncrementalWithIdentityOps(previous Index, changedPaths, deletedPaths []string, identityOps incrementalFileIdentityOps) (idx Index, ok bool) {
	if !incrementalUpdateShapeSupported(previous, changedPaths, deletedPaths) {
		return Index{}, false
	}
	identityOps = incrementalIdentityOpsWithDefaults(identityOps)
	p, err := identityOps.loadProject(previous.Project.Root)
	if err != nil {
		return Index{}, false
	}
	return updateApexFilesIncrementalWithLoadedProject(previous, changedPaths, deletedPaths, identityOps, p)
}

func incrementalUpdateShapeSupported(previous Index, changedPaths, deletedPaths []string) bool {
	if len(previous.Diagnostics) != 0 {
		return false
	}
	deleted := pathSet(deletedPaths)
	changed := pathSet(changedPaths)
	if len(changed) > 1 || len(deleted) > 1 || len(changed)+len(deleted) == 0 {
		return false
	}
	for key := range changed {
		if deleted[key] {
			return false
		}
	}
	return true
}

func updateApexFilesIncrementalWithLoadedProject(previous Index, changedPaths, deletedPaths []string, identityOps incrementalFileIdentityOps, p project.Project) (idx Index, ok bool) {
	if !incrementalUpdateShapeSupported(previous, changedPaths, deletedPaths) {
		return Index{}, false
	}
	deleted := pathSet(deletedPaths)
	changed := pathSet(changedPaths)
	owners, ownersOK := incrementalSourceOwners(previous, p)
	if !ownersOK {
		return Index{}, false
	}
	var changedSource incrementalResolvedSource
	var deletedSource incrementalResolvedSource
	if len(changed) == 1 {
		for path := range changed {
			var resolved bool
			changedSource, resolved = resolveIncrementalSource(owners, path, true)
			if !resolved {
				return Index{}, false
			}
		}
	}
	if len(deleted) == 1 {
		for path := range deleted {
			if _, err := identityOps.lstat(path); !os.IsNotExist(err) {
				return Index{}, false
			}
			var resolved bool
			deletedSource, resolved = resolveIncrementalSource(owners, path, false)
			if !resolved {
				return Index{}, false
			}
		}
	}
	if len(changed) == 1 && len(deleted) == 1 {
		if !sameIncrementalSourceMetadata(changedSource.owner.metadata, deletedSource.owner.metadata) || changedSource.packagePath != deletedSource.packagePath || !strings.EqualFold(filepath.Ext(changedSource.path), filepath.Ext(deletedSource.path)) {
			return Index{}, false
		}
	}

	idx = Index{
		Project:               previous.Project,
		Objects:               previous.Objects,
		CustomMetadataRecords: previous.CustomMetadataRecords,
		CodeIntelSymbols:      previous.CodeIntelSymbols,
		CodeIntelUses:         previous.CodeIntelUses,
		Dependencies:          previous.Dependencies,
		projectIdentity:       previous.projectIdentity,
		sourceDigests:         previous.sourceDigests,
		apexMetadataInputs:    cloneApexMetadataInputs(previous.apexMetadataInputs),
	}
	for path := range deleted {
		idx.sourceDigests = idx.sourceDigests.withoutSource(path)
		idx.apexMetadataInputs = cloneApexMetadataInputsWithoutSource(idx.apexMetadataInputs, path)
	}
	identityPathByRequestedKey := make(map[string]string)
	ambiguousIdentity := false
	recordFileIdentity := func(key, path string) {
		if len(changed) == 0 {
			return
		}
		if existing, exists := identityPathByRequestedKey[key]; exists {
			if existing != path {
				ambiguousIdentity = true
			}
			return
		}
		identityPathByRequestedKey[key] = path
	}
	sameTypeMetadata := func(typ TypeSymbol, metadata incrementalSourceMetadata) bool {
		return typ.Namespace == metadata.namespace && typ.SourceRoot == metadata.root && typ.Version == metadata.version && typ.Dependency == metadata.dependency && reflect.DeepEqual(typ.SourceNamespaceRemaps, metadata.namespaceRemaps)
	}
	sameTriggerMetadata := func(trigger TriggerSymbol, metadata incrementalSourceMetadata) bool {
		return trigger.Namespace == metadata.namespace && trigger.SourceRoot == metadata.root && trigger.Version == metadata.version && trigger.Dependency == metadata.dependency && reflect.DeepEqual(trigger.SourceNamespaceRemaps, metadata.namespaceRemaps)
	}
	knownChanged := false
	knownDeleted := false
	oldChangedTypes := 0
	oldDeletedTypes := 0
	oldChangedTriggers := 0
	oldDeletedTriggers := 0
	previousTriggerKeys := make(map[string]bool, len(previous.Triggers))
	for _, trigger := range previous.Triggers {
		key := namespaceTypeKey(trigger.Namespace, trigger.Name)
		if previousTriggerKeys[key] {
			return Index{}, false
		}
		previousTriggerKeys[key] = true
	}
	for _, typ := range previous.Types {
		key := cleanFilePath(typ.File)
		isSource := typ.HasSourceSnapshot()
		if !deleted[key] && isSource {
			recordFileIdentity(key, typ.File)
		}
		if changed[key] {
			if !isSource || !sameTypeMetadata(typ, changedSource.owner.metadata) {
				return Index{}, false
			}
			knownChanged = true
			oldChangedTypes++
		}
		if deleted[key] {
			if !isSource || !sameTypeMetadata(typ, deletedSource.owner.metadata) {
				return Index{}, false
			}
			knownDeleted = true
			oldDeletedTypes++
		}
	}
	for _, trigger := range previous.Triggers {
		key := cleanFilePath(trigger.File)
		isSource := trigger.HasSourceSnapshot()
		if !deleted[key] && isSource {
			recordFileIdentity(key, trigger.File)
		}
		if changed[key] {
			if !isSource || !sameTriggerMetadata(trigger, changedSource.owner.metadata) {
				return Index{}, false
			}
			knownChanged = true
			oldChangedTriggers++
		}
		if deleted[key] {
			if !isSource || !sameTriggerMetadata(trigger, deletedSource.owner.metadata) {
				return Index{}, false
			}
			knownDeleted = true
			oldDeletedTriggers++
		}
	}
	if len(deleted) == 1 && !knownDeleted {
		return Index{}, false
	}
	if len(changed) == 1 && len(deleted) == 1 && knownChanged {
		return Index{}, false
	}
	if len(changed) == 1 {
		key := cleanFilePath(changedSource.path)
		recordFileIdentity(key, changedSource.path)
		if ambiguousIdentity {
			return Index{}, false
		}
		identityByRequestedKey := make(map[string]os.FileInfo, len(identityPathByRequestedKey))
		for requestedKey, path := range identityPathByRequestedKey {
			info, err := identityOps.stat(path)
			if err != nil {
				return Index{}, false
			}
			identityByRequestedKey[requestedKey] = info
		}
		identity, exists := identityByRequestedKey[key]
		if !exists {
			return Index{}, false
		}
		for requestedKey, candidateIdentity := range identityByRequestedKey {
			if requestedKey != key && identityOps.sameFile(identity, candidateIdentity) {
				return Index{}, false
			}
		}
	}

	seenTypes := make(map[string]TypeSymbol)
	for _, typ := range previous.Types {
		if deleted[cleanFilePath(typ.File)] || changed[cleanFilePath(typ.File)] {
			continue
		}
		key := namespaceTypeKey(typ.Namespace, typ.Name)
		if _, exists := seenTypes[key]; exists {
			return Index{}, false
		}
		seenTypes[key] = typ
		idx.Types = append(idx.Types, typ)
	}
	seenTriggers := make(map[string]bool, len(previous.Triggers)+1)
	for _, trigger := range previous.Triggers {
		if deleted[cleanFilePath(trigger.File)] || changed[cleanFilePath(trigger.File)] {
			continue
		}
		seenTriggers[namespaceTypeKey(trigger.Namespace, trigger.Name)] = true
		idx.Triggers = append(idx.Triggers, trigger)
	}
	newTypes := 0
	newTriggers := 0
	if len(changed) == 1 {
		path := changedSource.path
		data, err := os.ReadFile(path) // #nosec G304 -- path is the single changed source already admitted to the loaded project index.
		if err != nil {
			return Index{}, false
		}
		idx.sourceDigests = idx.sourceDigests.withSourceDigest(path, data)
		metadata := changedSource.owner.metadata
		capturedMetadata, effectiveAPIVersion := captureApexMetadataInput(path, metadata.apiVersion)
		if !sourceStillMatches(path, sha256.Sum256(data)) {
			return Index{}, false
		}
		if !metadataStillMatches(path, capturedMetadata) {
			return Index{}, false
		}
		if idx.apexMetadataInputs == nil {
			idx.apexMetadataInputs = make(map[sourceOccurrenceKey]ApexMetadataInput)
		}
		idx.apexMetadataInputs[sourceOccurrenceKeyForMetadata(SourceMetadata{RequestedPath: path, Root: metadata.root, Namespace: metadata.namespace, Version: metadata.version, Dependency: metadata.dependency, NamespaceRemaps: metadata.namespaceRemaps})] = capturedMetadata
		source := project.NormalizeApexNamespaceTokens(string(data), metadata.namespace)
		source = namespaceremap.ApplySource(metadata.namespaceRemaps, source)
		parser := apexast.NewParser()
		file := parser.ParseSource(path, source)
		if len(file.Diagnostics) > 0 {
			return Index{}, false
		}
		for _, decl := range file.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
				for _, sym := range typeSymbolsFromDeclaration(path, decl, "", 0, false) {
					sym.Namespace = metadata.namespace
					sym.SourceNamespaceRemaps = cloneIncrementalNamespaceRemaps(metadata.namespaceRemaps)
					sym.SourceRoot = metadata.root
					sym.Version = metadata.version
					sym.EffectiveAPIVersion = effectiveAPIVersion
					sym.SourceBacked = true
					sym.Dependency = metadata.dependency
					key := namespaceTypeKey(sym.Namespace, sym.Name)
					if _, exists := seenTypes[key]; exists {
						return Index{}, false
					} else {
						seenTypes[key] = sym
					}
					idx.Types = append(idx.Types, sym)
					newTypes++
				}
			case apexast.DeclarationTrigger:
				trigger := TriggerSymbol{
					Name:                  decl.Name,
					Namespace:             metadata.namespace,
					SourceNamespaceRemaps: cloneIncrementalNamespaceRemaps(metadata.namespaceRemaps),
					SourceRoot:            metadata.root,
					Version:               metadata.version,
					EffectiveAPIVersion:   effectiveAPIVersion,
					SourceBacked:          true,
					ObjectName:            decl.ObjectName,
					Events:                decl.Events,
					File:                  path,
					Dependency:            metadata.dependency,
					BodyRange:             decl.BodyRange,
					Range:                 decl.Range,
				}
				triggerKey := namespaceTypeKey(trigger.Namespace, trigger.Name)
				if seenTriggers[triggerKey] {
					return Index{}, false
				}
				seenTriggers[triggerKey] = true
				idx.Triggers = append(idx.Triggers, trigger)
				newTriggers++
			}
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".cls" {
			if newTypes == 0 || newTriggers != 0 {
				return Index{}, false
			}
		} else if extension == ".trigger" {
			if newTypes != 0 || newTriggers != 1 {
				return Index{}, false
			}
		} else {
			return Index{}, false
		}
	}
	if len(changed) == 1 && len(deleted) == 0 && knownChanged && changedSource.owner.metadata.dependency && oldChangedTypes != newTypes {
		return Index{}, false
	}
	if len(changed) == 1 && len(deleted) == 0 && knownChanged && oldChangedTriggers != newTriggers {
		return Index{}, false
	}
	if len(changed) == 1 && len(deleted) == 1 && (oldDeletedTypes != newTypes || oldDeletedTriggers != newTriggers) {
		return Index{}, false
	}
	dependencyIndex := -1
	dependencyTypeDelta := 0
	if len(changed) == 1 && changedSource.owner.metadata.dependency {
		dependencyIndex = changedSource.owner.dependencyIndex
		dependencyTypeDelta += newTypes - oldChangedTypes
	}
	if len(deleted) == 1 && deletedSource.owner.metadata.dependency {
		if dependencyIndex != -1 && dependencyIndex != deletedSource.owner.dependencyIndex {
			return Index{}, false
		}
		dependencyIndex = deletedSource.owner.dependencyIndex
		dependencyTypeDelta -= oldDeletedTypes
	}
	if dependencyTypeDelta != 0 {
		if dependencyIndex < 0 || dependencyIndex >= len(previous.Dependencies) || previous.Dependencies[dependencyIndex].ApexTypes+dependencyTypeDelta < 0 {
			return Index{}, false
		}
		idx.Dependencies = append([]DependencyInfo(nil), previous.Dependencies...)
		idx.Dependencies[dependencyIndex].ApexTypes += dependencyTypeDelta
	}
	sort.Slice(idx.Types, func(i, j int) bool {
		if idx.Types[i].Namespace != idx.Types[j].Namespace {
			return idx.Types[i].Namespace < idx.Types[j].Namespace
		}
		return idx.Types[i].Name < idx.Types[j].Name
	})
	sort.Slice(idx.Triggers, func(i, j int) bool {
		if idx.Triggers[i].Namespace != idx.Triggers[j].Namespace {
			return idx.Triggers[i].Namespace < idx.Triggers[j].Namespace
		}
		return idx.Triggers[i].Name < idx.Triggers[j].Name
	})
	return idx, true
}

func cloneApexMetadataInputs(in map[sourceOccurrenceKey]ApexMetadataInput) map[sourceOccurrenceKey]ApexMetadataInput {
	out := make(map[sourceOccurrenceKey]ApexMetadataInput, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneApexMetadataInputsWithoutSource(in map[sourceOccurrenceKey]ApexMetadataInput, path string) map[sourceOccurrenceKey]ApexMetadataInput {
	cleaned := cleanFilePath(path)
	out := make(map[sourceOccurrenceKey]ApexMetadataInput, len(in))
	for key, value := range in {
		if cleanFilePath(key.requestedPath) == cleaned {
			continue
		}
		out[key] = value
	}
	return out
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

func typeSymbolsFromDeclaration(path string, decl apexast.Declaration, parent string, parentDepth int, parentIsTest bool) []TypeSymbol {
	sym := typeSymbolFromDeclaration(path, decl, parent, parentDepth, parentIsTest)
	sym.SuperClass = decl.SuperClass
	sym.Interfaces = append([]string(nil), decl.Interfaces...)
	out := []TypeSymbol{sym}
	for _, member := range decl.Members {
		switch member.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			out = append(out, typeSymbolsFromDeclaration(path, member, sym.Name, sym.NestingDepth, sym.IsTest)...)
		}
	}
	return out
}

func typeSymbolFromDeclaration(path string, decl apexast.Declaration, parent string, parentDepth int, parentIsTest bool) TypeSymbol {
	localName := decl.Name
	name := localName
	ownerName := ""
	nestingDepth := 0
	if parent != "" {
		name = parent + "." + localName
		ownerName = parent
		nestingDepth = parentDepth + 1
	}
	sym := TypeSymbol{
		Kind:           decl.Kind,
		Name:           name,
		LocalName:      localName,
		OwnerName:      ownerName,
		NestingDepth:   nestingDepth,
		File:           path,
		SourceBacked:   true,
		Modifiers:      decl.Modifiers,
		Annotations:    decl.Annotations,
		TypeParameters: append([]string(nil), decl.TypeParameters...),
		IsTest:         parentIsTest || hasTestModifier(decl.Modifiers),
		Range:          decl.Range,
	}
	for _, member := range decl.Members {
		if member.Name == "" {
			continue
		}
		if member.Kind == apexast.DeclarationMethod && hasModifier(member.Modifiers, "testmethod") {
			sym.IsTest = true
		}
		sym.Members = append(sym.Members, MemberSymbol{
			Kind:        member.Kind,
			Name:        member.Name,
			Type:        member.Type,
			Modifiers:   member.Modifiers,
			Annotations: member.Annotations,
			Parameters:  member.Parameters,
			Accessors:   member.Accessors,
			HasBody:     member.HasBody,
			BodyRange:   member.BodyRange,
			IsTest:      hasTestModifier(member.Modifiers) || (member.Kind == apexast.DeclarationMethod && hasModifier(member.Modifiers, "testmethod")),
			Range:       member.Range,
		})
	}
	return sym
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
	severity := diagnostic.Warning
	if sameOwnerDuplicate(current, previous) {
		severity = diagnostic.Error
	}
	return diagnostic.Diagnostic{
		Severity: severity,
		Code:     "GLADETYPE001",
		Message:  fmt.Sprintf("duplicate top-level symbol %q; first seen in %s", current.Name, previous.File),
		File:     current.File,
		Range:    &current.Range,
	}
}

func sameOwnerDuplicate(current, previous TypeSymbol) bool {
	if cleanFilePath(current.File) != cleanFilePath(previous.File) {
		return false
	}
	return strings.EqualFold(current.OwnerName, previous.OwnerName)
}
