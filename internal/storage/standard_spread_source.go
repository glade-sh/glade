package storage

import (
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Source-backed lazy spread
//
// An org with StandardSpread set holds only the standard catalog objects that
// something has referenced. Every other object the eager spread would hold is
// installed on first reference from a process-wide, memoized template, with the
// definition shared by reference as runtime clones share it. Installing a
// template does not clear the runtime schema stamp: the org's logical schema is
// the complete spread, and the template is exactly what that spread holds.
// Every route that enumerates the schema completes the spread first
// (MaterializeStandardObjectSpread) so no stamp-keyed cache sees a partial set.
const (
	standardSpreadPlain  = "plain"
	standardSpreadShaped = "shaped"
)

// StandardSpreadSourceBacked reports whether the org installs catalog objects
// from the shared source.
func (o *OrgState) StandardSpreadSourceBacked() bool {
	return o != nil && o.StandardSpread != ""
}

// EnableStandardSpreadSource marks an empty or partly built org as lazily
// spread. The shared RecordType object exists in a complete spread, so it is
// installed first.
func (o *OrgState) EnableStandardSpreadSource() {
	if o == nil {
		return
	}
	o.ensureFeatureProfile()
	if o.Objects == nil {
		o.Objects = make(map[string]ObjectState)
	}
	o.StandardSpread = standardSpreadPlain
	if _, ok := o.Objects["RecordType"]; !ok {
		ensureRecordTypeObject(o)
	}
	// Hand-written code reads these objects by literal name, so they are
	// present from the start, as the eager spread has them.
	for _, name := range standardSpreadCoreObjects {
		if !standardSpreadIncludes(name) {
			continue
		}
		if _, exists := o.Objects[name]; !exists {
			ensureStandardObjectWithProfile(o, name, nil)
		}
	}
}

func (o *OrgState) markStandardSpreadShaped() {
	if o != nil && o.StandardSpread == standardSpreadPlain {
		o.StandardSpread = standardSpreadShaped
	}
}

type spreadTemplateKey struct {
	phase      string
	signature  string
	multiCurr  bool
	recordType bool
	object     string
}

type spreadTemplateCell struct {
	done  atomic.Bool
	once  sync.Once
	state ObjectState
	ok    bool
	// evicted marks a tombstone: the group this cell belonged to was dropped
	// for the field budget. A tombstone holds no state.
	evicted bool
	// replacement marks a cell that took a tombstone's place. An untouched
	// check waits for it, as it does for a rebuilt plain cell, so an eviction
	// never changes an answer.
	replacement bool
	// group is the group that counts this cell's fields. It is set when the
	// cell is built, before done, and is nil for a cell built outside
	// buildSpreadTemplateCell.
	group *spreadTemplateGroup
}

// spreadTemplateGroupKey names a group: the templates of one phase and feature
// shape.
type spreadTemplateGroupKey struct {
	phase      string
	signature  string
	multiCurr  bool
	recordType bool
}

// spreadTemplateGroup records when a group's cells were last used. The live
// field total is a hint: the eviction pass recomputes it from the map before it
// drops anything.
type spreadTemplateGroup struct {
	lastUse atomic.Uint64
}

func (k spreadTemplateKey) group() spreadTemplateGroupKey {
	return spreadTemplateGroupKey{phase: k.phase, signature: k.signature, multiCurr: k.multiCurr, recordType: k.recordType}
}

// spreadTemplateTombstone stands in for every evicted cell. It carries no
// state, so an evicted key costs one map entry.
var spreadTemplateTombstone = &spreadTemplateCell{evicted: true}

var (
	spreadTemplateGroups     sync.Map // spreadTemplateGroupKey -> *spreadTemplateGroup
	spreadTemplateClock      atomic.Uint64
	spreadTemplateLiveFields atomic.Int64
	spreadTemplateEvictMu    sync.Mutex
)

func spreadGroupFor(key spreadTemplateGroupKey) *spreadTemplateGroup {
	if g, ok := spreadTemplateGroups.Load(key); ok {
		return g.(*spreadTemplateGroup)
	}
	g, _ := spreadTemplateGroups.LoadOrStore(key, &spreadTemplateGroup{})
	return g.(*spreadTemplateGroup)
}

// touch records a use of the cell's group. It reads group only after done, so
// a cell another goroutine is still building is skipped.
func (c *spreadTemplateCell) touch() {
	if !c.done.Load() {
		return
	}
	if g := c.group; g != nil {
		g.lastUse.Store(spreadTemplateClock.Add(1))
	}
}

// liveSpreadTemplateCell returns the cell for key, creating it when absent.
// A tombstone left by an eviction is replaced with a fresh cell, so callers
// never see one. Goroutines that meet the same tombstone share the winner.
func liveSpreadTemplateCell(key spreadTemplateKey) *spreadTemplateCell {
	for {
		cellAny, ok := spreadTemplates.Load(key)
		if !ok {
			cellAny, _ = spreadTemplates.LoadOrStore(key, &spreadTemplateCell{})
		}
		cell := cellAny.(*spreadTemplateCell)
		if !cell.evicted {
			return cell
		}
		fresh := &spreadTemplateCell{replacement: true}
		if spreadTemplates.CompareAndSwap(key, cell, fresh) {
			return fresh
		}
	}
}

// buildSpreadTemplateOnce builds the cell if no goroutine has, then counts it.
func buildSpreadTemplateOnce(cell *spreadTemplateCell, key spreadTemplateKey, orgID string, features []string, hasRecordType bool) {
	built := false
	cell.once.Do(func() {
		built = true
		buildSpreadTemplateCell(cell, key, orgID, features, hasRecordType)
	})
	cell.done.Store(true)
	if built {
		// Count the fields after done. A scan then either sees the cell and
		// counts it, or skips it and leaves this addition in place; it never
		// loses it. The worst case is one cell counted twice until the next
		// pass corrects the hint.
		if cell.ok {
			spreadTemplateLiveFields.Add(int64(len(cell.state.Definition.Fields)))
		}
		noteSpreadTemplateBuilt(key)
	}
}

// enforceSpreadTemplateBudget evicts least recently used groups, other than the
// caller's, while the live template fields exceed the budget. An evicted
// group's cells become tombstones; a later use builds the template again from
// the same key and gets an equal value. Holders of an evicted template keep it.
func enforceSpreadTemplateBudget(caller spreadTemplateGroupKey) {
	budget := spreadTemplateFieldBudget.Load()
	if budget <= 0 || spreadTemplateLiveFields.Load() <= budget {
		return
	}
	spreadTemplateEvictMu.Lock()
	defer spreadTemplateEvictMu.Unlock()
	observed := spreadTemplateLiveFields.Load()
	sums := map[spreadTemplateGroupKey]int64{}
	var total, evicted int64
	spreadTemplates.Range(func(k, v any) bool {
		cell := v.(*spreadTemplateCell)
		if cell.evicted || !cell.done.Load() || !cell.ok {
			return true
		}
		n := int64(len(cell.state.Definition.Fields))
		sums[k.(spreadTemplateKey).group()] += n
		total += n
		return true
	})
	for total > budget {
		var victim spreadTemplateGroupKey
		var oldest uint64
		found := false
		for key, n := range sums {
			if n == 0 || key == caller {
				continue
			}
			if use := spreadGroupFor(key).lastUse.Load(); !found || use < oldest {
				victim, oldest, found = key, use, true
			}
		}
		if !found {
			break
		}
		spreadTemplates.Range(func(k, v any) bool {
			if k.(spreadTemplateKey).group() == victim {
				if cell := v.(*spreadTemplateCell); !cell.evicted {
					spreadTemplates.CompareAndSwap(k, cell, spreadTemplateTombstone)
				}
			}
			return true
		})
		total -= sums[victim]
		evicted += sums[victim]
		sums[victim] = 0
		spreadTemplateGroups.Delete(victim)
	}
	// Replace the hint with the exact total only if no build counted fields
	// during the scan. Otherwise subtract what was evicted and keep their
	// increments; the next pass corrects any drift.
	if !spreadTemplateLiveFields.CompareAndSwap(observed, total) {
		spreadTemplateLiveFields.Add(-evicted)
	}
}

// defaultSpreadTemplateFieldBudget is the live template field count that
// triggers eviction of the least recently used group, about 280 MB.
const defaultSpreadTemplateFieldBudget = 200_000

var spreadTemplateFieldBudget atomic.Int64

func init() { spreadTemplateFieldBudget.Store(defaultSpreadTemplateFieldBudget) }

// SetSpreadTemplateFieldBudget sets how many template fields may stay live
// across all template groups and returns the previous budget. A value of zero
// or less turns eviction off.
func SetSpreadTemplateFieldBudget(fields int) int {
	return int(spreadTemplateFieldBudget.Swap(int64(fields)))
}

var spreadTemplates sync.Map // spreadTemplateKey -> *spreadTemplateCell

// spreadTemplate returns the state the eager spread holds for a catalog object
// at this org's phase and feature shape. The result is shared: callers install
// a frozen copy and never write to it.
func spreadTemplate(org *OrgState, canonical string) (ObjectState, bool) {
	_, hasRecordType := org.Objects["RecordType"]
	key := spreadTemplateKey{
		phase:      org.StandardSpread,
		signature:  canonicalFeatureSignature(org.Features),
		multiCurr:  org.HasFeature("MultiCurrency"),
		recordType: hasRecordType,
		object:     canonical,
	}
	cell := liveSpreadTemplateCell(key)
	buildSpreadTemplateOnce(cell, key, org.OrgID, org.Features, hasRecordType)
	cell.touch()
	return cell.state, cell.ok
}

// noteSpreadTemplateBuilt counts a built template and releases the decoded
// catalog every spreadTemplateReleaseEvery builds. A lazily growing org decodes
// catalog members one by one; this bounds what they leave behind. A later
// template decodes again, same bytes. Plain templates stay: dropping them here
// would make each rebuilt cell decode again.
func noteSpreadTemplateBuilt(key spreadTemplateKey) {
	if StandardCatalogReleaseEnabled() && spreadTemplatesBuilt.Add(1)%spreadTemplateReleaseEvery == 0 {
		releaseDecodedCatalog()
	}
	enforceSpreadTemplateBudget(key.group())
}

// spreadTemplateReleaseEvery is how many built templates pass between catalog
// releases, which bounds the decoded catalog to about that many objects.
const spreadTemplateReleaseEvery = 512

var spreadTemplatesBuilt atomic.Uint64

// droppedPlainTemplates remembers the keys of plain-phase cells that
// dropPlainSpreadTemplates removed, so SpreadObjectUntouched can rebuild one on
// demand and answer as it did before the drop.
var droppedPlainTemplates struct {
	mu   sync.Mutex
	keys map[spreadTemplateKey]struct{}
}

// buildSpreadTemplateCell fills a cell with the template for key. A template
// is a pure function of its key and the org id.
func buildSpreadTemplateCell(cell *spreadTemplateCell, key spreadTemplateKey, orgID string, features []string, hasRecordType bool) {
	scratch := OrgState{
		OrgID:    orgID,
		Features: slices.Clone(features),
		Objects:  make(map[string]ObjectState),
	}
	if hasRecordType {
		ensureRecordTypeObject(&scratch)
	}
	before := len(scratch.Objects)
	ensureStandardObjectWithProfile(&scratch, key.object, nil)
	if key.phase == standardSpreadShaped {
		applyShapeToLazyObject(&scratch, key.object)
	}
	// A template is only safe when ensuring the object touched nothing
	// else: no other object and no record type row.
	if len(scratch.Objects) != before+1 || len(scratch.Objects["RecordType"].Records) != 0 {
		return
	}
	cell.state = scratch.Objects[key.object]
	cell.ok = true
	cell.group = spreadGroupFor(key.group())
}

// dropPlainSpreadTemplates removes each plain-phase template at most once,
// after a shaped group exists. Later plain orgs can rebuild and retain it.
func dropPlainSpreadTemplates() {
	shaped := false
	spreadTemplates.Range(func(k, _ any) bool {
		if k.(spreadTemplateKey).phase == standardSpreadShaped {
			shaped = true
			return false
		}
		return true
	})
	if !shaped {
		return
	}
	droppedPlainTemplates.mu.Lock()
	defer droppedPlainTemplates.mu.Unlock()
	spreadTemplates.Range(func(k, _ any) bool {
		key := k.(spreadTemplateKey)
		if key.phase != standardSpreadPlain {
			return true
		}
		if _, alreadyDropped := droppedPlainTemplates.keys[key]; alreadyDropped {
			// A live plain org may have rebuilt this cell. Keep it so later
			// completed spreads do not repeatedly decode the same catalog.
			return true
		}
		if droppedPlainTemplates.keys == nil {
			droppedPlainTemplates.keys = map[spreadTemplateKey]struct{}{}
		}
		droppedPlainTemplates.keys[key] = struct{}{}
		spreadTemplates.Delete(k)
		return true
	})
}

// rebuiltPlainTemplate returns the cell for a key that dropPlainSpreadTemplates
// removed, rebuilding it. It reports false for a key that never had a cell.
func rebuiltPlainTemplate(key spreadTemplateKey, org OrgState, hasRecordType bool) (*spreadTemplateCell, bool) {
	droppedPlainTemplates.mu.Lock()
	_, dropped := droppedPlainTemplates.keys[key]
	droppedPlainTemplates.mu.Unlock()
	if !dropped {
		return nil, false
	}
	cell := liveSpreadTemplateCell(key)
	buildSpreadTemplateOnce(cell, key, org.OrgID, org.Features, hasRecordType)
	return cell, true
}

// spreadObjectOwed resolves a name to a catalog object this lazy org still
// owes: known, absent, in the spread, and allowed once the shape applied.
func spreadObjectOwed(org *OrgState, name string) (string, bool) {
	if org == nil || org.StandardSpread == "" {
		return "", false
	}
	canonical, ok := ResolveKnownStandardObjectName(name)
	if !ok || !standardSpreadIncludes(canonical) {
		return "", false
	}
	if _, exists := org.Objects[canonical]; exists {
		return "", false
	}
	if org.StandardSpread == standardSpreadShaped && !org.AllowsStandardObject(canonical) {
		return "", false
	}
	return canonical, true
}

// SpreadOwedObjectNames lists, in name order, the canonical name of every
// catalog object a lazy org still owes: what MaterializeStandardObjectSpread
// would add. With the installed names it is the object set of the completed
// spread, so a names-only enumeration can answer without installing. It
// returns nil for an org that is not lazy. It only reads the org.
func SpreadOwedObjectNames(org *OrgState) []string {
	if !org.LazyStandardSpreadEnabled() {
		return nil
	}
	var owed []string
	seen := make(map[string]struct{})
	for _, name := range KnownStandardObjectNames() {
		canonical, ok := LazyStandardObjectName(org, name)
		if !ok {
			continue
		}
		// Two catalog spellings can fold to one canonical object.
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		owed = append(owed, canonical)
	}
	sort.Strings(owed)
	return owed
}

// installSpreadObject installs one owed object from the shared source. It
// reports false when the object is not owed or has no safe template; callers
// then complete the spread so the answer still matches an eager org.
func installSpreadObject(org *OrgState, name string) (string, bool) {
	return installSpreadObjectWithPlan(org, name, nil)
}

// installSpreadObjectWithPlan is installSpreadObject with the key prefix plan
// supplied by a caller that installs many objects. A nil plan is computed on
// demand when the org's prefixes are validated.
func installSpreadObjectWithPlan(org *OrgState, name string, plan map[string]string) (string, bool) {
	canonical, ok := spreadObjectOwed(org, name)
	if !ok {
		return "", false
	}
	template, ok := spreadTemplate(org, canonical)
	if !ok {
		return "", false
	}
	installed := template.CloneRuntimeFrozenDefinition()
	if org.spreadPlanApplies() {
		// The org's prefixes are already unique; the object arrives with the
		// prefix the full assignment gives it.
		if plan == nil {
			plan = spreadKeyPrefixPlan(org)
		}
		if prefix, ok := plan[canonical]; ok {
			installed.Definition.KeyPrefix = prefix
		}
	}
	org.Objects[canonical] = installed
	return canonical, true
}

// materializeSpreadSource installs every owed object, then marks the org
// complete. It never clears the stamp.
func materializeSpreadSource(org *OrgState) bool {
	if org == nil || org.StandardSpread == "" {
		return false
	}
	// One plan serves every install: only catalog objects arrive, and the plan
	// does not depend on them.
	var plan map[string]string
	if org.spreadPlanApplies() {
		plan = spreadKeyPrefixPlan(org)
	}
	for _, name := range KnownStandardObjectNames() {
		if _, owed := spreadObjectOwed(org, name); !owed {
			continue
		}
		if _, ok := installSpreadObjectWithPlan(org, name, plan); !ok {
			// No safe template: build this object in place, as the eager
			// spread does.
			ensureStandardObjectWithProfile(org, name, nil)
			if org.StandardSpread == standardSpreadShaped {
				applyShapeToLazyObject(org, name)
			}
		}
	}
	org.StandardSpread = ""
	// Every template of this shape is built and the decoded members have no
	// reader left for this org.
	ReleaseStandardCatalogCaches()
	return true
}

// Key prefixes
//
// EnsureUniqueKeyPrefixes gives every object whose prefix conflicts the next
// free custom prefix in name order, so an object's final prefix depends on the
// whole name set. A lazy org computes that assignment over the names it holds
// plus the names it owes, using each owed object's catalog prefix, and applies
// it to what it installs. The result is memoized by what can change it: the
// feature shape and the objects outside the catalog.
var spreadPrefixPlans struct {
	mu    sync.Mutex
	plans map[string]spreadPrefixPlanEntry
}

// spreadPrefixPlanEntry holds the seed map its key names by address. Holding it
// keeps the address from being reused by another map while the entry is cached,
// and lookups compare identity as well.
type spreadPrefixPlanEntry struct {
	seed map[string]string
	plan map[string]string
}

// spreadPlanKeyBuilds counts plan lookups, for tests that bound the work.
var spreadPlanKeyBuilds atomic.Int64

// maxSpreadPrefixPlans bounds the memo: a long-lived process sees few feature
// shapes and custom-object sets, and a miss only recomputes.
const maxSpreadPrefixPlans = 64

func spreadCatalogName(name string) bool {
	canonical, ok := ResolveKnownStandardObjectName(name)
	return ok && canonical == name && standardSpreadIncludes(name)
}

func spreadKeyPrefixPlan(org *OrgState) map[string]string {
	spreadPlanKeyBuilds.Add(1)
	var key strings.Builder
	key.WriteString(org.StandardSpread)
	key.WriteByte(0)
	key.WriteString(canonicalFeatureSignature(org.Features))
	key.WriteByte(0)
	key.WriteString(strconv.FormatUint(uint64(reflect.ValueOf(org.spreadPrefixSeed).Pointer()), 16))
	others := make([]string, 0, 16)
	for name, state := range org.Objects {
		if spreadCatalogName(name) {
			continue
		}
		others = append(others, name+"\x00"+state.Definition.KeyPrefix+"\x00"+strconv.FormatBool(IsCustomSettingDefinition(state.Definition)))
	}
	sort.Strings(others)
	for _, entry := range others {
		key.WriteByte(1)
		key.WriteString(entry)
	}
	cacheKey := key.String()
	spreadPrefixPlans.mu.Lock()
	cached, ok := spreadPrefixPlans.plans[cacheKey]
	spreadPrefixPlans.mu.Unlock()
	if ok && reflect.ValueOf(cached.seed).Pointer() == reflect.ValueOf(org.spreadPrefixSeed).Pointer() {
		return cached.plan
	}
	entries := make([]keyPrefixEntry, 0, len(org.Objects)+len(KnownStandardObjectNames()))
	seen := make(map[string]bool, len(org.Objects))
	for name, state := range org.Objects {
		seen[name] = true
		if spreadCatalogName(name) {
			entries = append(entries, keyPrefixEntry{name: name, prefix: state.Definition.KeyPrefix})
			continue
		}
		entries = append(entries, keyPrefixEntry{name: name, prefix: state.Definition.KeyPrefix, customSetting: IsCustomSettingDefinition(state.Definition)})
	}
	for _, name := range KnownStandardObjectNames() {
		if seen[name] {
			continue
		}
		if _, owed := spreadObjectOwed(org, name); owed {
			prefix, assignedBefore := org.spreadPrefixSeed[name]
			if !assignedBefore {
				prefix = StandardKeyPrefix(name)
			}
			entries = append(entries, keyPrefixEntry{name: name, prefix: prefix})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	assigned := assignUniqueKeyPrefixes(entries)
	plan := make(map[string]string, len(entries))
	for i, entry := range entries {
		plan[entry.name] = assigned[i]
	}
	spreadPrefixPlans.mu.Lock()
	defer spreadPrefixPlans.mu.Unlock()
	if existing, ok := spreadPrefixPlans.plans[cacheKey]; ok && reflect.ValueOf(existing.seed).Pointer() == reflect.ValueOf(org.spreadPrefixSeed).Pointer() {
		return existing.plan
	}
	if spreadPrefixPlans.plans == nil || len(spreadPrefixPlans.plans) >= maxSpreadPrefixPlans {
		spreadPrefixPlans.plans = make(map[string]spreadPrefixPlanEntry)
	}
	spreadPrefixPlans.plans[cacheKey] = spreadPrefixPlanEntry{seed: org.spreadPrefixSeed, plan: plan}
	return plan
}

// applySpreadKeyPrefixPlan is EnsureUniqueKeyPrefixes for a lazy org.
func applySpreadKeyPrefixPlan(org *OrgState) {
	plan := spreadKeyPrefixPlan(org)
	org.spreadPrefixSeed = plan
	changed := false
	for name, state := range org.Objects {
		prefix, ok := plan[name]
		if !ok || state.Definition.KeyPrefix == prefix {
			continue
		}
		state.Definition.KeyPrefix = prefix
		org.Objects[name] = state
		changed = true
	}
	if changed {
		org.ClearRuntimeSchemaStamp()
	}
}

// SpreadObjectUntouched reports whether a lazy org holds a catalog object
// exactly as the shared source installed it: the template's definition, no
// records and no indexes. Such an object is a pure function of the org's
// feature shape, so a fingerprint of the org's ambient state may leave it out;
// an eager org holds the same object, and a lazy org that has not installed it
// yet would otherwise differ only by when something touched it.
func (o OrgState) SpreadObjectUntouched(name string) bool {
	if o.StandardSpread == "" {
		return false
	}
	state, ok := o.Objects[name]
	if !ok || len(state.Records) != 0 || len(state.Indexes) != 0 {
		return false
	}
	_, hasRecordType := o.Objects["RecordType"]
	key := spreadTemplateKey{
		phase:      o.StandardSpread,
		signature:  canonicalFeatureSignature(o.Features),
		multiCurr:  o.HasFeature("MultiCurrency"),
		recordType: hasRecordType,
		object:     name,
	}
	var cell *spreadTemplateCell
	if cellAny, ok := spreadTemplates.Load(key); ok {
		cell = cellAny.(*spreadTemplateCell)
		if cell.evicted {
			// The group was evicted for the budget. The template is a pure
			// function of the key, so build it again and compare as usual.
			cell = liveSpreadTemplateCell(key)
			buildSpreadTemplateOnce(cell, key, o.OrgID, o.Features, hasRecordType)
		}
		cell.touch()
	} else if rebuilt, ok := rebuiltPlainTemplate(key, o, hasRecordType); ok {
		cell = rebuilt
	} else {
		return false
	}
	template, built := cell.peek()
	if !built && cell.replacement {
		// Another goroutine is building the cell that replaced a tombstone.
		// Wait for it so an eviction does not change the answer.
		buildSpreadTemplateOnce(cell, key, o.OrgID, o.Features, hasRecordType)
		template, built = cell.peek()
	}
	if !built {
		// A replacement for a dropped plain cell may already be building.
		// Wait for it, but preserve the nonblocking answer for other cells.
		if rebuilt, dropped := rebuiltPlainTemplate(key, o, hasRecordType); dropped {
			template, built = rebuilt.peek()
		}
		if !built {
			return false
		}
	}
	if reflect.ValueOf(template.Definition.Fields).Pointer() == reflect.ValueOf(state.Definition.Fields).Pointer() {
		return true
	}
	// A deep clone of the org holds equal, unshared definitions. The key
	// prefix is the one value the full assignment may have replaced.
	want := template.Definition
	want.KeyPrefix = state.Definition.KeyPrefix
	return reflect.DeepEqual(want, state.Definition)
}

// peek returns the template if it has been built, without building it.
func (c *spreadTemplateCell) peek() (ObjectState, bool) {
	if !c.done.Load() {
		return ObjectState{}, false
	}
	return c.state, c.ok
}

// SpreadKeyPrefixOwner returns the catalog object a source-backed lazy org
// still owes whose key prefix, once installed, is prefix: the first such
// object in name order, which is the object a complete spread reports. When
// the org's prefixes have been validated an object installs with the prefix
// the unique assignment gives it, and a conflicting catalog prefix may have
// moved, so the answer follows the same plan the install applies. It returns
// "" when no owed object ends up with the prefix.
func (o *OrgState) SpreadKeyPrefixOwner(prefix string) string {
	if o == nil || o.StandardSpread == "" {
		return ""
	}
	if !o.spreadPlanApplies() {
		owner := StandardKeyPrefixOwner(prefix)
		if owner == "" {
			return ""
		}
		if canonical, owed := spreadObjectOwed(o, owner); owed {
			return canonical
		}
		return ""
	}
	owner := ""
	for name, assigned := range spreadKeyPrefixPlan(o) {
		if assigned != prefix || (owner != "" && name >= owner) {
			continue
		}
		if _, owed := spreadObjectOwed(o, name); owed {
			owner = name
		}
	}
	return owner
}

// spreadPlanApplies reports whether objects install with the prefix the unique
// assignment gives them: the org's prefixes are validated, or were before the
// schema stamp was cleared.
func (o *OrgState) spreadPlanApplies() bool {
	return o.keyPrefixesValidated || o.spreadPrefixesPlanned
}
