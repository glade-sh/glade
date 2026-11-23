package vm

import (
	"sort"
	"strings"
)

// classLookupOverlay records class registrations made on a VM whose class
// lookup was frozen, so that RegisterClass and the next FreezeClassLookup cost
// O(registered names) instead of rebuilding the lookup over every class.
//
// Between registration and the next freeze, lookups behave exactly as the
// unfrozen path that used to be rebuilt here: an exact Classes key first, then
// the last-registered class for each name (overrides), then the frozen
// generation the overlay started from, read with the values it held when the
// overlay began (prior). FreezeClassLookup then publishes a layer over the
// root generation holding only the canonical keys whose full-build result
// changed. A key that no longer resolves gets "" in the layer, hiding the root
// entry. The shared root generation is never written.
type classLookupOverlay struct {
	from *frozenClassLookup
	// written holds every Classes key written since the root generation,
	// including the layer the overlay started from.
	written map[string]struct{}
	// added lists Classes keys created since from, in write order.
	added []string
	// affected and affectedTop hold the canonical keys, and canonical
	// top-level names, whose result may differ from from.
	affected    map[string]struct{}
	affectedTop map[string]struct{}
	prior       map[string]Class
	overrides   map[string]Class
}

func (f *frozenClassLookup) rootGeneration() *frozenClassLookup {
	if f.base != nil {
		return f.base
	}
	return f
}

// resolve returns the Classes key a full build of this generation binds to
// typeName.
func (f *frozenClassLookup) resolve(typeName string) (string, bool) {
	if f.base != nil {
		alias, ok := f.keys[typeName]
		if !ok {
			alias, ok = foldLookupStringMap(f.keys, typeName)
		}
		if ok {
			return alias, alias != ""
		}
		f = f.base
	}
	alias, ok := f.keys[typeName]
	if !ok {
		alias, ok = foldLookupStringMap(f.keys, typeName)
	}
	return alias, ok
}

// buildContributors indexes, once per root generation, every Classes key that
// writes each canonical key in a full build. classes must hold the root
// generation's names and class identities, which every VM still on the root
// generation without class value writes does.
func (f *frozenClassLookup) buildContributors(classes map[string]Class) {
	f.contributorsOnce.Do(func() {
		out := make(map[string][]string, len(f.keys))
		add := func(name, alias string) {
			key := canonicalClassLookupKey(name)
			list := out[key]
			if n := len(list); n > 0 && list[n-1] == alias {
				return
			}
			out[key] = append(list, alias)
		}
		for alias, class := range classes {
			add(alias, alias)
			add(class.Name, alias)
			if class.Namespace != "" {
				add(class.Namespace+"."+class.Name, alias)
			}
		}
		f.contributors = out
		f.contributorsReady.Store(true)
	})
}

// beginClassLookupOverlay returns the overlay registration writes through, or
// nil when this VM must use the full rebuild path. Registration calls it after
// prepareClassMapWrite, so vm.Classes holds this VM's class values.
func (vm *VM) beginClassLookupOverlay() *classLookupOverlay {
	if vm.classOverlay != nil {
		return vm.classOverlay
	}
	from := vm.frozenClassLookup
	if from == nil || vm.classValuesWritten || from.searchEntries == nil || from.topLevel == nil {
		return nil
	}
	root := from.rootGeneration()
	if root.copyPlan == nil {
		return nil
	}
	if from == root {
		root.buildContributors(vm.Classes)
	} else if !root.contributorsReady.Load() {
		return nil
	}
	overlay := &classLookupOverlay{
		from:        from,
		written:     make(map[string]struct{}, len(from.written)+4),
		affected:    make(map[string]struct{}),
		affectedTop: make(map[string]struct{}),
		prior:       make(map[string]Class),
		overrides:   make(map[string]Class),
	}
	for name := range from.written {
		overlay.written[name] = struct{}{}
	}
	vm.classOverlay = overlay
	vm.frozenClassLookup = nil
	vm.sharedClassCopyPlan = nil
	return overlay
}

// recordWrite runs before a registration writes next at alias.
func (o *classLookupOverlay) recordWrite(classes map[string]Class, alias string, next Class) {
	if old, existed := classes[alias]; existed {
		o.recordPrior(classes, alias)
		o.markAffected(alias, old)
	} else {
		o.added = append(o.added, alias)
	}
	o.written[alias] = struct{}{}
	o.markAffected(alias, next)
}

// recordPrior keeps the value alias held when the overlay began.
func (o *classLookupOverlay) recordPrior(classes map[string]Class, alias string) {
	if _, seen := o.prior[alias]; seen {
		return
	}
	if old, ok := classes[alias]; ok {
		o.prior[alias] = old
	}
}

func (o *classLookupOverlay) markAffected(alias string, class Class) {
	o.affected[canonicalClassLookupKey(alias)] = struct{}{}
	o.affected[canonicalClassLookupKey(class.Name)] = struct{}{}
	if class.Namespace != "" {
		o.affected[canonicalClassLookupKey(class.Namespace+"."+class.Name)] = struct{}{}
	}
	if name := strings.TrimSpace(class.Name); name != "" && !strings.Contains(name, ".") {
		o.affectedTop[canonicalClassLookupKey(name)] = struct{}{}
	}
}

// storeAlias is storeClassLookupAlias for the overlay: the last registration
// of a name wins until the next freeze.
func (o *classLookupOverlay) storeAlias(name string, class Class) {
	if strings.TrimSpace(name) == "" {
		return
	}
	o.overrides[canonicalClassLookupKey(name)] = class
}

func (o *classLookupOverlay) lookup(vm *VM, typeName string) (Class, bool) {
	if class, ok := foldLookupClassMap(o.overrides, typeName); ok {
		return class, true
	}
	alias, ok := o.from.resolve(typeName)
	if !ok {
		return Class{}, false
	}
	if class, ok := o.prior[alias]; ok {
		return class, true
	}
	return vm.classEntryRaw(alias)
}

// freezeClassLookupOverlay publishes the overlay as a layer over the root
// generation. It returns false when a class value write since the overlay
// began requires the full build. FreezeClassLookup folds class value overlays
// first, so vm.Classes holds this VM's class values.
func (vm *VM) freezeClassLookupOverlay(o *classLookupOverlay) bool {
	if vm.classValuesWritten {
		return false
	}
	from := o.from
	root := from.rootGeneration()
	written := sortedNameSet(o.written)
	// A full build binds a key only from aliases that write it, so each key
	// considers the written aliases that write it, in the same sorted order.
	writtenByKey, writtenByTop := vm.indexWrittenClassLookupKeys(written)
	keys := make(map[string]string, len(o.affected))
	if from != root {
		for key, alias := range from.keys {
			keys[key] = alias
		}
	}
	for key := range o.affected {
		previous, _ := from.resolve(key)
		alias, ok := vm.recomputeClassLookupKey(root.contributors[key], writtenByKey[key], key, previous)
		rootAlias, inRoot := root.keys[key]
		switch {
		case ok && inRoot && alias == rootAlias:
			delete(keys, key)
		case ok:
			keys[key] = alias
		case inRoot:
			keys[key] = ""
		default:
			delete(keys, key)
		}
	}
	plan := deriveClassCopyPlan(root.copyPlan, vm.Classes, o.written)
	if plan == nil {
		plan = buildClassCopyPlan(vm.Classes)
	}
	generation := nextClassLookupGeneration.Add(1)
	layer := &frozenClassLookup{
		generation:    generation,
		keys:          keys,
		base:          root,
		written:       o.written,
		searchEntries: mergeClassNameSearchEntries(from.searchEntries, o.added),
		topLevel:      vm.patchTopLevelClassLookup(from.topLevel, root.contributors, writtenByTop, o.affectedTop),
		copyPlan:      plan,
	}
	vm.frozenClassLookup = layer
	vm.sharedClassCopyPlan = plan
	vm.classMapWritten = false
	vm.classLookupGeneration = generation
	vm.classNameSearchCache = layer.searchEntries
	vm.topLevelClassLookup = layer.topLevel
	vm.resetClassLookupNameCache()
	vm.classLookup = nil
	return true
}

// indexWrittenClassLookupKeys lists, per canonical key, the written aliases
// whose class can bind that key (byKey: alias, name or namespace.name; byTop:
// a dotless top-level name), keeping written's order. An alias without a class
// binds nothing, as recomputeClassLookupKey and patchTopLevelClassLookup skip
// it. A listed alias is still checked exactly by the caller.
func (vm *VM) indexWrittenClassLookupKeys(written []string) (byKey, byTop map[string][]string) {
	byKey = make(map[string][]string, len(written))
	byTop = make(map[string][]string, len(written))
	for _, alias := range written {
		class, ok := vm.classEntryRaw(alias)
		if !ok {
			continue
		}
		seen := [3]string{}
		for i, name := range [3]string{alias, class.Name, classLookupQualifiedName(class)} {
			if i == 2 && class.Namespace == "" {
				break
			}
			key := canonicalClassLookupKey(name)
			if key == seen[0] && i > 0 || key == seen[1] && i > 1 {
				continue
			}
			seen[i] = key
			byKey[key] = append(byKey[key], alias)
		}
		if name := strings.TrimSpace(class.Name); name != "" && !strings.Contains(name, ".") {
			key := canonicalClassLookupKey(name)
			byTop[key] = append(byTop[key], alias)
		}
	}
	return byKey, byTop
}

func classLookupQualifiedName(class Class) string {
	if class.Namespace == "" {
		return ""
	}
	return class.Namespace + "." + class.Name
}

// recomputeClassLookupKey returns the Classes key a full FreezeClassLookup
// binds to key: the highest classLookupKeyRank, then the lowest namespace.
// Full ties are arbitrary there (map order); previous is kept when tied.
func (vm *VM) recomputeClassLookupKey(contributors, written []string, key, previous string) (string, bool) {
	best, bestNS, bestRank, found := "", "", 0, false
	consider := func(alias string) {
		class, ok := vm.classEntryRaw(alias)
		if !ok {
			return
		}
		rank, ok := classLookupRankAt(alias, class, key)
		if !ok {
			return
		}
		ns := strings.ToLower(strings.TrimSpace(class.Namespace))
		if found {
			if rank < bestRank {
				return
			}
			if rank == bestRank && (ns > bestNS || (ns == bestNS && (alias != previous || best == previous))) {
				return
			}
		}
		best, bestNS, bestRank, found = alias, ns, rank, true
	}
	for _, alias := range contributors {
		consider(alias)
	}
	for _, alias := range written {
		consider(alias)
	}
	return best, found
}

// classLookupRankAt is the best rank alias writes at key in a full build.
func classLookupRankAt(alias string, class Class, key string) (int, bool) {
	rank, ok := 0, false
	consider := func(name string, exact bool) {
		if !classLookupKeyEquals(name, key) {
			return
		}
		if candidate := classLookupKeyRank(class, exact); !ok || candidate > rank {
			rank, ok = candidate, true
		}
	}
	consider(alias, true)
	consider(class.Name, false)
	if class.Namespace != "" && classLookupQualifiedKeyEquals(class.Namespace, class.Name, key) {
		if candidate := classLookupKeyRank(class, true); !ok || candidate > rank {
			rank, ok = candidate, true
		}
	}
	return rank, ok
}

// classLookupQualifiedKeyEquals reports
// classLookupKeyEquals(namespace+"."+name, key) without building the string.
func classLookupQualifiedKeyEquals(namespace, name, key string) bool {
	if namespace == "" || name == "" || !classLookupPlainEdge(namespace[0]) || !classLookupPlainEdge(name[len(name)-1]) {
		return classLookupKeyEquals(namespace+"."+name, key)
	}
	// TrimSpace removes nothing here: both edges are non-space ASCII.
	if len(namespace)+1+len(name) != len(key) || key[len(namespace)] != '.' {
		return false
	}
	return classLookupFoldedEquals(namespace, key[:len(namespace)]) && classLookupFoldedEquals(name, key[len(namespace)+1:])
}

func classLookupPlainEdge(c byte) bool {
	return c < 0x80 && c != ' ' && c != '\t' && c != '\n' && c != '\v' && c != '\f' && c != '\r'
}

// classLookupFoldedEquals is classLookupKeyEquals without trimming.
func classLookupFoldedEquals(name, key string) bool {
	if len(name) != len(key) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != key[i] {
			return false
		}
	}
	return true
}

// classLookupKeyEquals reports canonicalClassLookupKey(name) == key without
// allocating.
func classLookupKeyEquals(name, key string) bool {
	name = strings.TrimSpace(name)
	if len(name) != len(key) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != key[i] {
			return false
		}
	}
	return true
}

// patchTopLevelClassLookup returns base with the affected names recomputed as
// rebuildTopLevelClassLookup would over vm.Classes. base is not modified.
func (vm *VM) patchTopLevelClassLookup(base map[string]topLevelClassLookup, contributors map[string][]string, written map[string][]string, affected map[string]struct{}) map[string]topLevelClassLookup {
	if len(affected) == 0 {
		return base
	}
	out := make(map[string]topLevelClassLookup, len(base)+len(affected))
	for key, entry := range base {
		out[key] = entry
	}
	for nameKey := range affected {
		var entry topLevelClassLookup
		found := false
		visit := func(alias string) {
			class, ok := vm.classEntryRaw(alias)
			if !ok {
				return
			}
			name := strings.TrimSpace(class.Name)
			if name == "" || strings.Contains(name, ".") || !classLookupKeyEquals(name, nameKey) {
				return
			}
			namespaceKey := canonicalClassLookupKey(class.Namespace)
			candidate := runtimeClassName(class)
			if entry.ByNamespace == nil {
				entry.ByNamespace = make(map[string]string)
			}
			if existing := entry.ByNamespace[namespaceKey]; existing == "" || strings.EqualFold(existing, candidate) {
				entry.ByNamespace[namespaceKey] = candidate
			}
			if entry.Unique == "" {
				entry.Unique = candidate
			} else if !strings.EqualFold(entry.Unique, candidate) {
				entry.Ambiguous = true
			}
			found = true
		}
		for _, alias := range contributors[nameKey] {
			visit(alias)
		}
		for _, alias := range written[nameKey] {
			visit(alias)
		}
		if found {
			out[nameKey] = entry
		} else {
			delete(out, nameKey)
		}
	}
	return out
}

// mergeClassNameSearchEntries returns base, sorted by Name, with added names
// inserted as classNameSearchEntries would order them. base is not modified.
func mergeClassNameSearchEntries(base []classNameSearchEntry, added []string) []classNameSearchEntry {
	if len(added) == 0 {
		return base
	}
	names := append([]string(nil), added...)
	sort.Strings(names)
	out := make([]classNameSearchEntry, 0, len(base)+len(names))
	i := 0
	for _, name := range names {
		for i < len(base) && base[i].Name < name {
			out = append(out, base[i])
			i++
		}
		out = append(out, classNameSearchEntry{Name: name, Lower: strings.ToLower(name)})
	}
	return append(out, base[i:]...)
}
