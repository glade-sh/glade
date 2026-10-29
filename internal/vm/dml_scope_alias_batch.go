package vm

import (
	"sort"
	"unsafe"

	"github.com/glade-sh/glade/internal/dml"
)

const dmlScopeAliasBatchMaxTargets = 200

type dmlScopeAliasMutation struct {
	previous aliasSnapshot
	updated  Value
}

// A prepared slot borrows existing backing. Publication never copies a holder,
// changes collection identity, or replaces a whole list with a trigger chunk.
type dmlScopeAliasSlot struct {
	dictionary map[string]Value
	key        string
	sequence   []Value
	index      int
	target     int
}

type dmlScopeAliasNode struct {
	original Value
	visiting bool
	changed  bool
}

type dmlScopeAliasSpan struct{ start, end uintptr }

type dmlScopeAliasPlan struct {
	previous     aliasSnapshot
	targets      map[methodReturnAliasTargetKey]int
	slots        [][]dmlScopeAliasSlot
	nodes        map[methodReturnAliasTargetKey]*dmlScopeAliasNode
	maps         map[uintptr]methodReturnAliasTargetKey
	spans        []dmlScopeAliasSpan
	changedRefs  map[methodReturnAliasTargetKey]bool
	changedMaps  map[uintptr]bool
	changedSpans []dmlScopeAliasSpan
}

func dmlFlatRecord(value Value, targets map[methodReturnAliasTargetKey]int) bool {
	if value.Kind != ValueObject || value.Ref == 0 {
		return false
	}
	for _, child := range value.Fields {
		// The legacy walker already prunes typed primitive bookkeeping maps.
		// Use that proof, rather than ignoring fields by an internal name.
		if child.Kind == ValueObject {
			if _, target := targets[methodReturnAliasTargetKey{ref: child.Ref, kind: child.Kind}]; target {
				return false
			}
			if child.classInstance || child.projectClass || !platformScalarObject(child.Type) || len(child.Fields) != 1 || child.Fields["value"].Kind != ValueString {
				return false
			}
		} else if !valueCannotContainAliasRef(child, 0, ValueObject) {
			return false
		}
	}
	return true
}

// Preparation and the population/publication loop have no Apex callbacks between
// them. Flat targets cannot introduce another target, a cycle, or a new slot.
// All ambiguous shapes fall back before the first population/publication.
func (vm *VM) prepareDMLScopeAliasPlan(scope map[string]Value, mutations []dmlScopeAliasMutation) *dmlScopeAliasPlan {
	reject := func(reason string) *dmlScopeAliasPlan {
		vm.recordDMLScopeAliasBatchFallback()
		if vm.scopeAliasTraversalObserver != nil {
			vm.scopeAliasTraversalObserver.scopeAliasBatchFallback(reason)
		}
		return nil
	}
	if len(mutations) < 2 {
		return nil
	}
	if len(mutations) > dmlScopeAliasBatchMaxTargets {
		return reject("too many mutation targets")
	}
	// Reject provenance and record shapes before allocating occurrence tables.
	// The target-aware pass below still rejects forged scalar cross-target views.
	for _, m := range mutations {
		if !m.previous.valid() || m.previous.kind != ValueObject || m.updated.Ref != m.previous.ref ||
			m.previous.typeName != mutations[0].previous.typeName || m.updated.Type != m.previous.typeName ||
			vm.localOnlyAlias(m.previous) || !dmlFlatRecord(m.updated, nil) {
			return reject("non-flat target or incomplete provenance")
		}
	}
	plan := &dmlScopeAliasPlan{
		previous: mutations[0].previous,
		targets:  make(map[methodReturnAliasTargetKey]int, len(mutations)), slots: make([][]dmlScopeAliasSlot, len(mutations)),
		nodes: make(map[methodReturnAliasTargetKey]*dmlScopeAliasNode), maps: make(map[uintptr]methodReturnAliasTargetKey),
		changedRefs: make(map[methodReturnAliasTargetKey]bool), changedMaps: make(map[uintptr]bool),
	}
	for i, m := range mutations {
		if !m.previous.valid() || m.previous.kind != ValueObject || m.updated.Ref != m.previous.ref ||
			m.previous.typeName != plan.previous.typeName || m.updated.Type != m.previous.typeName || vm.localOnlyAlias(m.previous) {
			return reject("non-flat target or incomplete provenance")
		}
		key := methodReturnAliasTargetKey{ref: m.previous.ref, kind: m.previous.kind}
		if _, ok := plan.targets[key]; ok {
			return reject("duplicate mutation target")
		}
		plan.targets[key] = i
	}
	for _, m := range mutations {
		if !dmlFlatRecord(m.updated, plan.targets) {
			return reject("non-flat mutation target")
		}
	}
	for name, root := range scope {
		if vm.scopeAliasTraversalObserver != nil {
			vm.scopeAliasTraversalObserver.scopeAliasRootVisited()
		}
		key := methodReturnAliasTargetKey{ref: root.Ref, kind: root.Kind}
		if _, ok := plan.targets[key]; ok {
			return reject("top-level caller alias")
		}
		if _, safe := plan.visit(root, dmlScopeAliasSlot{dictionary: scope, key: name}); !safe {
			return reject("unsafe caller graph")
		}
	}
	sort.Slice(plan.spans, func(i, j int) bool { return plan.spans[i].start < plan.spans[j].start })
	for i := 1; i < len(plan.spans); i++ {
		if plan.spans[i].start < plan.spans[i-1].end {
			return reject("overlapping caller slices")
		}
	}
	sort.Slice(plan.changedSpans, func(i, j int) bool { return plan.changedSpans[i].start < plan.changedSpans[j].start })
	seen := make(map[methodReturnAliasTargetKey]Value)
	checkScope := func(owner map[string]Value) bool {
		for _, root := range owner {
			if !plan.externalSafe(root, seen) {
				return false
			}
		}
		return true
	}
	if !sameMapBacking(scope, vm.Globals) && !checkScope(vm.Globals) {
		return reject("external caller sharing")
	}
	for _, owner := range vm.scopeStack {
		if !sameMapBacking(scope, owner) && !checkScope(owner) {
			return reject("external caller sharing")
		}
	}
	// Inspect each static graph once, rather than once per record or changed node.
	for _, class := range vm.ClassEntries() {
		for _, field := range class.StaticFields {
			if !plan.externalSafe(field.Value, seen) {
				return reject("external static sharing")
			}
		}
	}
	for _, entries := range vm.platformCache {
		for _, entry := range entries {
			if !plan.externalSafe(entry.Value, seen) {
				return reject("external cache sharing")
			}
		}
	}
	return plan
}

func dmlScopeAliasMaps(value Value) []map[string]Value {
	switch value.Kind {
	case ValueObject:
		return []map[string]Value{value.Fields}
	case ValueMap:
		return []map[string]Value{value.Map, value.MapKeys}
	}
	return nil
}

func dmlScopeAliasSequence(value Value) []Value {
	switch value.Kind {
	case ValueList:
		return value.List
	case ValueSet:
		return value.Set
	}
	return nil
}

func dmlScopeAliasRange(sequence []Value) dmlScopeAliasSpan {
	if len(sequence) == 0 {
		return dmlScopeAliasSpan{}
	}
	start := uintptr(unsafe.Pointer(unsafe.SliceData(sequence)))
	return dmlScopeAliasSpan{start, start + uintptr(len(sequence))*unsafe.Sizeof(Value{})}
}

func (plan *dmlScopeAliasPlan) visit(value Value, slot dmlScopeAliasSlot) (bool, bool) {
	key := methodReturnAliasTargetKey{ref: value.Ref, kind: value.Kind}
	if target, ok := plan.targets[key]; ok {
		if !dmlFlatRecord(value, plan.targets) {
			return false, false
		}
		slot.target = target
		plan.slots[target] = append(plan.slots[target], slot)
		return true, true
	}
	if valueCannotContainAliasRef(value, plan.previous.ref, ValueObject) {
		return false, true
	}
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return false, true
	case ValueObject, ValueMap, ValueList, ValueSet:
	default:
		return false, false
	}
	if value.Ref == 0 {
		return false, false
	}
	if node, ok := plan.nodes[key]; ok {
		// Equal Ref with a different view/backing cannot reuse prepared occurrences.
		if node.visiting || !sameMethodReturnAliasBatchBacking(value, node.original) ||
			!sameSliceHeaderBacking(dmlScopeAliasSequence(value), dmlScopeAliasSequence(node.original)) {
			return false, false
		}
		return node.changed, true
	}
	node := &dmlScopeAliasNode{original: value, visiting: true}
	plan.nodes[key] = node
	for mapIndex, dictionary := range dmlScopeAliasMaps(value) {
		if value.Kind == ValueMap && mapIndex == 1 && mapKeyTypeCannotContainAlias(value.Type, plan.previous) {
			continue
		}
		if dictionary == nil {
			continue
		}
		ptr := mapIdentityPointer(dictionary)
		if owner, ok := plan.maps[ptr]; ok && owner != key {
			return false, false
		}
		plan.maps[ptr] = key
		for name, child := range dictionary {
			changed, safe := plan.visit(child, dmlScopeAliasSlot{dictionary: dictionary, key: name})
			if !safe {
				return false, false
			}
			node.changed = node.changed || changed
		}
	}
	sequence := dmlScopeAliasSequence(value)
	span := dmlScopeAliasRange(sequence)
	if span.start != 0 {
		plan.spans = append(plan.spans, span)
	}
	for i, child := range sequence {
		changed, safe := plan.visit(child, dmlScopeAliasSlot{sequence: sequence, index: i})
		if !safe {
			return false, false
		}
		node.changed = node.changed || changed
	}
	node.visiting = false
	if node.changed {
		plan.changedRefs[key] = true
		for _, dictionary := range dmlScopeAliasMaps(value) {
			if dictionary != nil {
				plan.changedMaps[mapIdentityPointer(dictionary)] = true
			}
		}
		if span.start != 0 {
			plan.changedSpans = append(plan.changedSpans, span)
		}
	}
	return node.changed, true
}

func (plan *dmlScopeAliasPlan) externalSafe(value Value, seen map[methodReturnAliasTargetKey]Value) bool {
	key := methodReturnAliasTargetKey{ref: value.Ref, kind: value.Kind}
	if _, target := plan.targets[key]; target {
		return dmlFlatRecord(value, plan.targets)
	}
	if plan.changedRefs[key] {
		return false
	}
	for _, dictionary := range dmlScopeAliasMaps(value) {
		if dictionary != nil && plan.changedMaps[mapIdentityPointer(dictionary)] {
			return false
		}
	}
	span := dmlScopeAliasRange(dmlScopeAliasSequence(value))
	if span.start != 0 {
		i := sort.Search(len(plan.changedSpans), func(i int) bool { return plan.changedSpans[i].end > span.start })
		if i < len(plan.changedSpans) && plan.changedSpans[i].start < span.end {
			return false
		}
	}
	// Empty/reference-free platform roots cannot contain a target. Apply the
	// same leaf proof as the legacy walker before requiring reference identity.
	if valueCannotContainAliasRef(value, plan.previous.ref, ValueObject) {
		return true
	}
	switch value.Kind {
	case ValueObject, ValueMap, ValueList, ValueSet:
	default:
		return true
	}
	if old, ok := seen[key]; ok {
		return sameMethodReturnAliasBatchBacking(value, old)
	}
	if value.Ref == 0 {
		return false
	}
	seen[key] = value
	for _, dictionary := range dmlScopeAliasMaps(value) {
		for _, child := range dictionary {
			if !plan.externalSafe(child, seen) {
				return false
			}
		}
	}
	for _, child := range dmlScopeAliasSequence(value) {
		if !plan.externalSafe(child, seen) {
			return false
		}
	}
	return true
}

func (plan *dmlScopeAliasPlan) publish(index int, updated Value) {
	for _, slot := range plan.slots[index] {
		if slot.dictionary != nil {
			slot.dictionary[slot.key] = updated
		} else {
			slot.sequence[slot.index] = updated
		}
	}
}

// Test-facing differential entry points use the same preparation and publication.
func (vm *VM) tryPropagateDMLScopeAliasSnapshotMutationsToScope(scope map[string]Value, mutations []dmlScopeAliasMutation) bool {
	plan := vm.prepareDMLScopeAliasPlan(scope, mutations)
	if plan == nil {
		return false
	}
	for i, m := range mutations {
		plan.publish(i, m.updated)
	}
	return true
}

func (vm *VM) propagateSuccessfulDMLTargetAliases(mutations []dmlScopeAliasMutation) {
	plan := vm.prepareDMLScopeAliasPlan(vm.Globals, mutations)
	if plan != nil {
		vm.recordDMLScopeAliasBatchTraversal(len(mutations))
	}
	for i, m := range mutations {
		if plan != nil {
			plan.publish(i, m.updated)
		} else {
			vm.propagateAliasSnapshotToScope(vm.Globals, m.previous, m.updated)
		}
		vm.propagateAliasSnapshotToStatics(m.previous, m.updated)
	}
}

// Keep optional preparation out of the ordinary DML publication frame. Most
// calls have a whole-list identity or fewer than two successful rows and never
// need a mutation slice, occurrence table, or per-result index array.
func (vm *VM) tryPublishDMLScopeAliasBatch(targets []*Value, results []dml.Result, op string) bool {
	count := 0
	var typeName string
	for i := range results {
		if !results[i].Success || i >= len(targets) || targets[i] == nil {
			continue
		}
		target := targets[i]
		if target.Kind != ValueObject || target.Ref == 0 || (count > 0 && target.Type != typeName) ||
			vm.localOnlyAlias(aliasSnapshot{ref: target.Ref, kind: target.Kind}) || !dmlFlatRecord(*target, nil) {
			return false
		}
		typeName = target.Type
		count++
		if count > dmlScopeAliasBatchMaxTargets {
			return false
		}
	}
	if count < 2 || count > dmlScopeAliasBatchMaxTargets {
		return false
	}
	mutations := make([]dmlScopeAliasMutation, 0, count)
	for i := range results {
		if results[i].Success && i < len(targets) && targets[i] != nil {
			mutations = append(mutations, dmlScopeAliasMutation{previous: snapshotAlias(*targets[i]), updated: *targets[i]})
		}
	}
	plan := vm.prepareDMLScopeAliasPlan(vm.Globals, mutations)
	if plan == nil {
		return false
	}
	vm.recordDMLScopeAliasBatchTraversal(count)
	index := 0
	for i := range results {
		if results[i].Success && i < len(targets) && targets[i] != nil {
			previous := snapshotAlias(*targets[i])
			vm.populateDMLResultFields(targets[i], results[i:i+1], op)
			plan.publish(index, *targets[i])
			vm.propagateAliasSnapshotToStatics(previous, *targets[i])
			index++
		}
	}
	return true
}
