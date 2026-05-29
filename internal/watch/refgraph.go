package watch

import (
	"os"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
)

// RefGraph is a static dependency graph over Apex type declarations. An edge
// "A depends on B" means type A references type B (by name, superclass, or
// implemented interface), so a change to B may affect A and any test that
// reaches A. The graph is derived purely from source identifiers and declared
// relationships, so it needs no profiling or runtime instrumentation: it is a
// byproduct of the type index the watcher/daemon already maintains.
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

	isTest      map[string]struct{} // canonical names that are runnable test classes
	allTests    []string            // sorted runnable test class names
	nameByLower map[string]string   // lowercased type name -> canonical name
	canonByFile map[string]string   // clean file path -> canonical type name
}

// BuildReferenceGraph constructs a reference graph from the type index by
// reading each declared type's source file once and recording which known type
// names it references. This is the only place that reads every file; callers
// keep the result and refresh it incrementally via Refresh.
func BuildReferenceGraph(index typesys.Index) *RefGraph {
	g := &RefGraph{
		deps:        make(map[string]map[string]struct{}),
		dependents:  make(map[string]map[string]struct{}),
		isTest:      make(map[string]struct{}),
		nameByLower: make(map[string]string),
		canonByFile: make(map[string]string),
	}
	g.refreshLightMaps(index)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		g.rescanType(typ)
	}
	return g
}

// Refresh returns a graph reflecting the given changes. Modifications to known
// Apex files re-scan only those files; additions, deletions, or changes to
// unknown files trigger a full rebuild so cross-file edges for new symbols are
// never missed (the conservative direction). A nil receiver builds from scratch.
func (g *RefGraph) Refresh(index typesys.Index, changes []Change) *RefGraph {
	if g == nil || g.needsFullRebuild(changes) {
		return BuildReferenceGraph(index)
	}
	g.refreshLightMaps(index)
	for _, change := range changes {
		if change.Kind != FileKindApexClass {
			continue
		}
		path := cleanPath(change.Path)
		if change.Op == ChangeDeleted {
			g.removeFile(path)
			continue
		}
		name, ok := g.canonByFile[path]
		if !ok {
			continue
		}
		for _, typ := range index.Types {
			if typ.Name == name {
				g.rescanType(typ)
				break
			}
		}
	}
	return g
}

func (g *RefGraph) needsFullRebuild(changes []Change) bool {
	for _, change := range changes {
		if change.Kind != FileKindApexClass {
			continue
		}
		switch change.Op {
		case ChangeAdded, ChangeDeleted:
			return true
		default:
			if _, ok := g.canonByFile[cleanPath(change.Path)]; !ok {
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
	g.canonByFile = make(map[string]string, len(index.Types))
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		g.nameByLower[strings.ToLower(typ.Name)] = typ.Name
		if file := cleanPath(typ.File); file != "" {
			g.canonByFile[file] = typ.Name
		}
	}
	g.isTest = make(map[string]struct{})
	for _, tc := range apextest.Discover(index, apextest.Options{}) {
		g.isTest[tc.ClassName] = struct{}{}
	}
	g.allTests = sortedSet(g.isTest)
}

func (g *RefGraph) rescanType(typ typesys.TypeSymbol) {
	refs := make(map[string]struct{})
	g.addStructuralRef(typ.Name, typ.SuperClass, refs)
	for _, iface := range typ.Interfaces {
		g.addStructuralRef(typ.Name, iface, refs)
	}
	if file := cleanPath(typ.File); file != "" {
		if data, err := os.ReadFile(file); err == nil {
			g.collectIdentifierRefs(data, typ.Name, refs)
		}
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

// collectIdentifierRefs tokenizes source into identifier runs and records any
// that name a known type. Lowercasing into a reused buffer keeps the map probe
// allocation-free. Matches inside comments or strings only ever over-select,
// which is safe.
func (g *RefGraph) collectIdentifierRefs(data []byte, self string, refs map[string]struct{}) {
	var buf [256]byte
	n := len(data)
	for i := 0; i < n; {
		for i < n && !isIdentRefByte(data[i]) {
			i++
		}
		start := i
		for i < n && isIdentRefByte(data[i]) {
			i++
		}
		token := data[start:i]
		if len(token) == 0 || len(token) > len(buf) {
			continue
		}
		for k := 0; k < len(token); k++ {
			b := token[k]
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			buf[k] = b
		}
		if canon, ok := g.nameByLower[string(buf[:len(token)])]; ok && canon != self {
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
	name, ok := g.canonByFile[path]
	if !ok {
		return
	}
	g.setDeps(name, nil)
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

func isIdentRefByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
