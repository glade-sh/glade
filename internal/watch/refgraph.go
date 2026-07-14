package watch

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// CanRefreshAuthoritativeFallbackGraph limits clone-refresh to the fallback
// shape whose unchanged-file reference facts remain authoritative. Recovery
// from errors and topology-changing edits rebuild the complete graph.
func CanRefreshAuthoritativeFallbackGraph(previous, updated typesys.Index, changes []Change) bool {
	if len(previous.Diagnostics) == 0 || len(changes) == 0 {
		return false
	}
	for _, diag := range previous.Diagnostics {
		if diag.Severity != diagnostic.Warning {
			return false
		}
	}
	changedFiles := make(map[string]bool, len(changes))
	for _, change := range changes {
		if change.Kind != FileKindApexClass || change.Op != ChangeModified {
			return false
		}
		changedFiles[cleanPath(change.Path)] = false
	}
	changedNames := make(map[string]string)
	for _, typ := range previous.Types {
		if typ.Dependency || typ.Artifact {
			continue
		}
		file := cleanPath(typ.File)
		if _, changed := changedFiles[file]; changed {
			changedFiles[file] = true
			changedNames[strings.ToLower(typ.Name)] = file
		}
	}
	for _, typ := range previous.Types {
		changedFile, changedName := changedNames[strings.ToLower(typ.Name)]
		if changedName && (typ.File == "" || cleanPath(typ.File) != changedFile) {
			return false
		}
	}
	for _, covered := range changedFiles {
		if !covered {
			return false
		}
	}
	if !sameDeclarationNameMultiplicity(previous, updated) {
		return false
	}
	return true
}

func sameDeclarationNameMultiplicity(previous, updated typesys.Index) bool {
	counts := func(index typesys.Index) map[string]int {
		result := make(map[string]int, len(index.Types))
		for _, typ := range index.Types {
			result[strings.ToLower(typ.Name)]++
		}
		return result
	}
	before, after := counts(previous), counts(updated)
	if len(before) != len(after) {
		return false
	}
	for name, count := range before {
		if after[name] != count {
			return false
		}
	}
	return true
}

// AuthoritativeApexGraphChanges compares the complete retained source set of
// two indexes. Missing digest coverage fails closed so callers rebuild.
func AuthoritativeApexGraphChanges(previous, updated typesys.Index) ([]Change, bool) {
	previousFiles, ok := authoritativeApexFiles(previous)
	if !ok {
		return nil, false
	}
	updatedFiles, ok := authoritativeApexFiles(updated)
	if !ok || len(previousFiles) != len(updatedFiles) {
		return nil, false
	}
	paths := make([]string, 0, len(previousFiles))
	for path, kind := range previousFiles {
		if updatedFiles[path] != kind {
			return nil, false
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	changes := make([]Change, 0)
	for _, path := range paths {
		before, beforeOK := previous.SourceDigest(path)
		after, afterOK := updated.SourceDigest(path)
		if !beforeOK || !afterOK {
			return nil, false
		}
		if before != after {
			changes = append(changes, Change{Path: path, Op: ChangeModified, Kind: previousFiles[path]})
		}
	}
	return changes, true
}

func authoritativeApexFiles(index typesys.Index) (map[string]FileKind, bool) {
	files := make(map[string]FileKind)
	add := func(path string, kind FileKind) bool {
		path = cleanPath(path)
		if path == "" {
			return false
		}
		if previous, exists := files[path]; exists && previous != kind {
			return false
		}
		files[path] = kind
		return true
	}
	for _, typ := range index.Types {
		if !typ.Dependency && !typ.Artifact && isApexSourceFile(typ.File, ".cls") && !add(typ.File, FileKindApexClass) {
			return nil, false
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.Dependency && isApexSourceFile(trigger.File, ".trigger") && !add(trigger.File, FileKindApexTrigger) {
			return nil, false
		}
	}
	return files, true
}

// RefGraph is a static dependency graph over Apex type declarations. An edge
// "A depends on B" means type A references B through code intelligence,
// superclass, or implemented-interface facts, so a change to B may affect A and
// any test that reaches A. The graph needs no profiling or runtime
// instrumentation: it is a byproduct of the type index the watcher/daemon
// already maintains.
//
// Selection walks the reverse edges (dependents) from each changed type to find
// every test that transitively reaches it. This catches changes to deep helper
// classes that no test names directly, which the previous text-mention graph
// could only handle by running the whole suite.
type RefGraph struct {
	// deps maps a canonical type name to the set of canonical type names it
	// references (depends on). Retained so a single file can be re-scanned and
	// its reverse edges updated in place.
	deps map[string]map[string]struct{}
	// dependents maps a canonical type name to the set of canonical type names
	// that depend on it (the reverse of deps).
	dependents map[string]map[string]struct{}

	isTest       map[string]struct{}            // canonical names that are runnable test classes
	allTests     []string                       // sorted runnable test class names
	nameByLower  map[string]string              // lowercased type name -> canonical name
	canonsByFile map[string]map[string]struct{} // clean file path -> canonical type names
	resolvedFile map[string]struct{}            // clean Apex file paths represented in codeintel
}

// BuildReferenceGraph constructs a reference graph from the type index and the
// codeintel graph. Callers keep the result and refresh it via Refresh.
func BuildReferenceGraph(index typesys.Index) *RefGraph {
	g := &RefGraph{
		deps:         make(map[string]map[string]struct{}),
		dependents:   make(map[string]map[string]struct{}),
		isTest:       make(map[string]struct{}),
		nameByLower:  make(map[string]string),
		canonsByFile: make(map[string]map[string]struct{}),
		resolvedFile: make(map[string]struct{}),
	}
	g.refreshLightMaps(index)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		g.addStructuralRefs(typ)
	}
	g.addCodeintelRefs(index)
	return g
}

// Refresh returns a graph reflecting the given changes. It rebuilds from the
// current type index so codeintel references and reverse edges stay in step. A
// nil receiver builds from scratch.
func (g *RefGraph) Refresh(index typesys.Index, changes []Change) *RefGraph {
	if g == nil || g.needsFullRebuild(changes) {
		return BuildReferenceGraph(index)
	}
	newCanonsByFile := canonicalTypesByFile(index)
	for _, change := range changes {
		file := cleanPath(change.Path)
		if !sameStringSet(g.canonsByFile[file], newCanonsByFile[file]) {
			return BuildReferenceGraph(index)
		}
	}
	oldCanonsByFile := g.canonsByFile
	g.refreshLightMaps(index)
	changedFiles := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.Kind != FileKindApexClass {
			continue
		}
		file := cleanPath(change.Path)
		delete(g.resolvedFile, file)
		for oldName := range oldCanonsByFile[file] {
			g.setDeps(oldName, nil)
		}
		for _, typ := range index.Types {
			if typ.Dependency || cleanPath(typ.File) != file {
				continue
			}
			g.addStructuralRefs(typ)
		}
		changedFiles = append(changedFiles, file)
	}
	if len(changedFiles) > 0 {
		g.addCodeintelRefsForFiles(index, changedFiles)
	}
	return g
}

// Refreshed returns an owned refreshed graph without mutating g. Watch state
// transactions use it to keep the currently published graph immutable until
// the replacement project, index, and graph can be published together.
func (g *RefGraph) Refreshed(index typesys.Index, changes []Change) *RefGraph {
	if g == nil {
		return BuildReferenceGraph(index)
	}
	return g.clone().Refresh(index, changes)
}

func (g *RefGraph) clone() *RefGraph {
	cloneSets := func(source map[string]map[string]struct{}) map[string]map[string]struct{} {
		result := make(map[string]map[string]struct{}, len(source))
		for key, values := range source {
			copied := make(map[string]struct{}, len(values))
			for value := range values {
				copied[value] = struct{}{}
			}
			result[key] = copied
		}
		return result
	}
	cloneSet := func(source map[string]struct{}) map[string]struct{} {
		result := make(map[string]struct{}, len(source))
		for key := range source {
			result[key] = struct{}{}
		}
		return result
	}
	cloneStrings := func(source map[string]string) map[string]string {
		result := make(map[string]string, len(source))
		for key, value := range source {
			result[key] = value
		}
		return result
	}
	return &RefGraph{
		deps:         cloneSets(g.deps),
		dependents:   cloneSets(g.dependents),
		isTest:       cloneSet(g.isTest),
		allTests:     append([]string(nil), g.allTests...),
		nameByLower:  cloneStrings(g.nameByLower),
		canonsByFile: cloneSets(g.canonsByFile),
		resolvedFile: cloneSet(g.resolvedFile),
	}
}

func (g *RefGraph) needsFullRebuild(changes []Change) bool {
	for _, change := range changes {
		if change.Kind != FileKindApexClass {
			return true
		}
		switch change.Op {
		case ChangeAdded, ChangeDeleted:
			return true
		default:
			if _, ok := g.canonsByFile[cleanPath(change.Path)]; !ok {
				return true
			}
		}
	}
	return false
}

// refreshLightMaps recomputes the index-derived maps (name lookup, file lookup,
// and test membership). All are O(types) and read no files.
func (g *RefGraph) refreshLightMaps(index typesys.Index) {
	g.nameByLower = make(map[string]string, len(index.Types))
	g.canonsByFile = make(map[string]map[string]struct{}, len(index.Types))
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		g.nameByLower[strings.ToLower(typ.Name)] = typ.Name
		if file := cleanPath(typ.File); file != "" {
			if g.canonsByFile[file] == nil {
				g.canonsByFile[file] = make(map[string]struct{})
			}
			g.canonsByFile[file][typ.Name] = struct{}{}
		}
	}
	g.isTest = make(map[string]struct{})
	for _, tc := range apextest.Discover(index, apextest.Options{}) {
		g.isTest[tc.ClassName] = struct{}{}
	}
	g.allTests = sortedSet(g.isTest)
}

func (g *RefGraph) addStructuralRefs(typ typesys.TypeSymbol) {
	refs := make(map[string]struct{})
	g.addStructuralRef(typ.Name, typ.SuperClass, refs)
	for _, iface := range typ.Interfaces {
		g.addStructuralRef(typ.Name, iface, refs)
	}
	g.setDeps(typ.Name, refs)
}

func (g *RefGraph) addStructuralRef(self, raw string, refs map[string]struct{}) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	if canon, ok := g.nameByLower[strings.ToLower(raw)]; ok && canon != self {
		refs[canon] = struct{}{}
		return
	}
	if dot := strings.LastIndex(raw, "."); dot >= 0 {
		if canon, ok := g.nameByLower[strings.ToLower(raw[dot+1:])]; ok && canon != self {
			refs[canon] = struct{}{}
		}
	}
}

// setDeps replaces the dependency set for name, keeping the reverse-edge map in
// sync by removing stale dependents and adding the new ones.
func (g *RefGraph) setDeps(name string, refs map[string]struct{}) {
	for old := range g.deps[name] {
		if set := g.dependents[old]; set != nil {
			delete(set, name)
			if len(set) == 0 {
				delete(g.dependents, old)
			}
		}
	}
	if len(refs) == 0 {
		delete(g.deps, name)
	} else {
		g.deps[name] = refs
	}
	for dep := range refs {
		set := g.dependents[dep]
		if set == nil {
			set = make(map[string]struct{})
			g.dependents[dep] = set
		}
		set[name] = struct{}{}
	}
}

func (g *RefGraph) removeFile(path string) {
	for name := range g.canonsByFile[path] {
		g.setDeps(name, nil)
	}
}

func canonicalTypesByFile(index typesys.Index) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{}, len(index.Types))
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		file := cleanPath(typ.File)
		if file == "" {
			continue
		}
		if result[file] == nil {
			result[file] = make(map[string]struct{})
		}
		result[file][typ.Name] = struct{}{}
	}
	return result
}

func sameStringSet(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, ok := right[value]; !ok {
			return false
		}
	}
	return true
}

func (g *RefGraph) addCodeintelRefs(index typesys.Index) {
	files := make([]string, 0, len(index.Types)+len(index.Triggers))
	seen := make(map[string]struct{})
	add := func(path, extension string) {
		if !isApexSourceFile(path, extension) {
			return
		}
		path = cleanPath(path)
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		files = append(files, path)
	}
	for _, typ := range index.Types {
		if !typ.Dependency && !typ.Artifact {
			add(typ.File, ".cls")
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.Dependency {
			add(trigger.File, ".trigger")
		}
	}
	sort.Strings(files)
	g.addCodeintelRefsForFiles(index, files)
}

func isApexSourceFile(path, extension string) bool {
	return path != "" && strings.EqualFold(filepath.Ext(path), extension)
}

func (g *RefGraph) addCodeintelRefsForFiles(index typesys.Index, files []string) {
	cg := codeintel.BuildApexReferences(index, files)
	typeByFile := make(map[string]string, len(index.Types))
	for _, typ := range index.Types {
		if typ.Dependency || typ.File == "" {
			continue
		}
		typeByFile[cleanPath(typ.File)] = typ.Name
	}
	for _, use := range cg.Uses {
		if !use.Resolved || use.Kind == codeintel.UseDeclaration {
			continue
		}
		from := typeByFile[cleanPath(use.File)]
		if from == "" {
			continue
		}
		g.resolvedFile[cleanPath(use.File)] = struct{}{}
		to := g.typeNameForCodeintelUse(cg, use.SymbolID)
		if to == "" || to == from {
			continue
		}
		refs := cloneDeps(g.deps[from])
		refs[to] = struct{}{}
		g.setDeps(from, refs)
	}
	for _, symbol := range cg.Symbols {
		if symbol.Kind == codeintel.SymbolApexType && symbol.File != "" {
			g.resolvedFile[cleanPath(symbol.File)] = struct{}{}
		}
	}
}

func (g *RefGraph) typeNameForCodeintelUse(cg codeintel.Graph, id codeintel.SymbolID) string {
	symbol, ok := cg.Definition(id)
	if !ok {
		return ""
	}
	switch symbol.Kind {
	case codeintel.SymbolApexType:
		if canon, ok := g.nameByLower[strings.ToLower(symbol.Name)]; ok {
			return canon
		}
	case codeintel.SymbolApexMember:
		owner := symbol.Metadata["owner"]
		if owner == "" && symbol.Container != "" {
			if container, ok := cg.Definition(symbol.Container); ok {
				owner = container.Name
			}
		}
		if owner == "" {
			parts := codeintel.ParseID(symbol.ID)
			if len(parts) >= 5 {
				owner = parts[3]
			}
		}
		if canon, ok := g.nameByLower[strings.ToLower(owner)]; ok {
			return canon
		}
	}
	return ""
}

func cloneDeps(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in)+1)
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}

// affectedTests returns the sorted set of test classes that transitively depend
// on name (including name itself when it is a test class).
func (g *RefGraph) affectedTests(name string) []string {
	out := make(map[string]struct{})
	visited := map[string]struct{}{name: {}}
	queue := []string{name}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, ok := g.isTest[cur]; ok {
			out[cur] = struct{}{}
		}
		for dependent := range g.dependents[cur] {
			if _, seen := visited[dependent]; !seen {
				visited[dependent] = struct{}{}
				queue = append(queue, dependent)
			}
		}
	}
	return sortedKeys(out)
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
