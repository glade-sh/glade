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
	cellAny, _ := spreadTemplates.LoadOrStore(key, &spreadTemplateCell{})
	cell := cellAny.(*spreadTemplateCell)
	cell.once.Do(func() {
		scratch := OrgState{
			OrgID:    org.OrgID,
			Features: slices.Clone(org.Features),
			Objects:  make(map[string]ObjectState),
		}
		if hasRecordType {
			ensureRecordTypeObject(&scratch)
		}
		before := len(scratch.Objects)
		ensureStandardObjectWithProfile(&scratch, canonical, nil)
		if key.phase == standardSpreadShaped {
			applyShapeToLazyObject(&scratch, canonical)
		}
		// A template is only safe when ensuring the object touched nothing
		// else: no other object and no record type row.
		if len(scratch.Objects) != before+1 || len(scratch.Objects["RecordType"].Records) != 0 {
			return
		}
		cell.state = scratch.Objects[canonical]
		cell.ok = true
	})
	cell.done.Store(true)
	return cell.state, cell.ok
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
	if org.keyPrefixesValidated {
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
	if org.keyPrefixesValidated {
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
	cellAny, ok := spreadTemplates.Load(spreadTemplateKey{
		phase:      o.StandardSpread,
		signature:  canonicalFeatureSignature(o.Features),
		multiCurr:  o.HasFeature("MultiCurrency"),
		recordType: hasRecordType,
		object:     name,
	})
	if !ok {
		return false
	}
	cell := cellAny.(*spreadTemplateCell)
	template, built := cell.peek()
	if !built {
		return false
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
	if !o.keyPrefixesValidated {
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
