package storage

import (
	"sort"
	"sync"
)

// EnsureStandardObjectSpread materializes the full standard schema catalog,
// the starting point of every `glade test` org. Network and NetworkMember wait
// for the Communities setting. Callers apply the project feature shape
// afterward, which removes any object the declared features turn off.
func EnsureStandardObjectSpread(org *OrgState) {
	if org == nil {
		return
	}
	for _, objectName := range KnownStandardObjectNames() {
		if !standardSpreadIncludes(objectName) {
			continue
		}
		EnsureStandardObject(org, objectName)
	}
}

// standardSpreadIncludes reports whether the spread seeds a catalog object.
// Network and NetworkMember arrive with the NetworksEnabled setting through
// the org shape instead.
func standardSpreadIncludes(objectName string) bool {
	return objectName != "Network" && objectName != "NetworkMember"
}

// EnableLazyStandardSpread marks an org whose standard catalog objects are
// added on first reference instead of at construction. `glade exec` uses it so
// anonymous Apex sees the same standard objects as `glade test` without
// building the whole spread for every call. The org's feature shape must
// already be applied: each lazily added object takes the org's feature profile,
// exactly as the eager spread does after ApplyProjectOrgShape.
func (o *OrgState) EnableLazyStandardSpread() {
	if o == nil {
		return
	}
	o.ensureFeatureProfile()
	o.lazyStandardSpread = true
}

// LazyStandardSpreadEnabled reports whether catalog objects are still pending
// for this org. MaterializeStandardObjectSpread clears it.
func (o *OrgState) LazyStandardSpreadEnabled() bool {
	return o != nil && (o.lazyStandardSpread || o.StandardSpread != "")
}

// LazyStandardObjectName resolves a name to a catalog object the lazy spread
// would still add: a known standard object the org shape allows and that is
// not installed yet. The result is the canonical API name.
func LazyStandardObjectName(org *OrgState, name string) (string, bool) {
	if org != nil && org.StandardSpread != "" {
		return spreadObjectOwed(org, name)
	}
	if org == nil || !org.lazyStandardSpread {
		return "", false
	}
	canonical, ok := ResolveKnownStandardObjectName(name)
	if !ok || !standardSpreadIncludes(canonical) {
		return "", false
	}
	if _, exists := org.Objects[canonical]; exists {
		return "", false
	}
	if !org.AllowsStandardObject(canonical) {
		return "", false
	}
	return canonical, true
}

// EnsureLazyStandardObject installs one pending catalog object and reports its
// canonical name. It is a no-op for names the lazy spread would not add.
//
// The object goes through the same two steps as in an eager org: the plain
// catalog definition the spread seeds, then the refresh with the org's feature
// profile that ApplyProjectOrgShape performs. One direct ensure with the
// profile is not the same definition for every object.
func EnsureLazyStandardObject(org *OrgState, name string) (string, bool) {
	canonical, ok := LazyStandardObjectName(org, name)
	if !ok {
		return "", false
	}
	if org.StandardSpread != "" {
		if installed, ok := installSpreadObject(org, canonical); ok {
			return installed, true
		}
		ensureStandardObjectWithProfile(org, canonical, nil)
		if org.StandardSpread == standardSpreadShaped {
			applyShapeToLazyObject(org, canonical)
		}
		return canonical, true
	}
	ensureStandardObjectWithProfile(org, canonical, nil)
	applyShapeToLazyObject(org, canonical)
	return canonical, true
}

// applyShapeToLazyObject repeats, for one object, what ApplyProjectOrgShape did
// to every object of an eager org: the whole-org feature appliers, the field
// overlay refresh for the org's profile, and record type rows.
func applyShapeToLazyObject(org *OrgState, objectName string) {
	if org.HasFeature("MultiCurrency") {
		applyMultiCurrencyField(org, objectName)
	}
	refreshObjectFeatureFields(org, objectName)
	if _, ok := org.Objects["RecordType"]; ok {
		if object := org.Objects[objectName]; len(object.Definition.RecordTypes) > 0 {
			ensureRecordTypeRecordsForObject(org, objectName)
		}
	}
}

// refreshObjectFeatureFields is refreshOrgFeatureFields for one object.
func refreshObjectFeatureFields(org *OrgState, objectName string) {
	object, ok := org.Objects[objectName]
	if !ok || !standardObjectFieldsNeedWrite(object.Definition, canonicalFeatureSignature(org.Features)) {
		return
	}
	object.Definition = object.Definition.Clone()
	EnsureStandardObjectFieldsForFeatures(&object.Definition, org.Features)
	org.Objects[objectName] = object
	org.ClearRuntimeSchemaStamp()
}

// MaterializeStandardObjectSpread completes a lazy org: every catalog object
// the shape allows is installed, disallowed gated objects stay out, and record
// type rows exist for the added objects. Callers that enumerate the whole
// schema (global describe, child relationship describes, relationship name
// lookups that missed) use it so their answers match an eagerly built org.
// It returns false when the org was not lazy or is already complete.
func MaterializeStandardObjectSpread(org *OrgState) bool {
	if org != nil && org.StandardSpread != "" {
		return materializeSpreadSource(org)
	}
	if org == nil || !org.lazyStandardSpread {
		return false
	}
	org.lazyStandardSpread = false
	// Plain catalog definitions first, then the shape steps in the order an
	// eager org applied them to every object: whole-org feature appliers,
	// gated objects, the field overlay refresh and record type rows. Objects
	// the lean org already shaped are left alone.
	added := make([]string, 0, len(KnownStandardObjectNames()))
	for _, objectName := range KnownStandardObjectNames() {
		if !standardSpreadIncludes(objectName) {
			continue
		}
		if _, exists := org.Objects[objectName]; exists {
			continue
		}
		ensureStandardObjectWithProfile(org, objectName, nil)
		added = append(added, objectName)
	}
	if org.HasFeature("MultiCurrency") {
		for _, name := range added {
			applyMultiCurrencyField(org, name)
		}
	}
	refreshOrgFeatureObjects(org)
	refreshOrgFeatureFields(org)
	if _, ok := org.Objects["RecordType"]; ok {
		ensureRecordTypeRecords(org)
	}
	return true
}

// StandardSpreadObjectsOf lists the spread's catalog objects an org already
// holds, sorted. A lazy org seeds exactly these from the catalog before its
// project metadata and platform fixtures apply, so their definitions match an
// org that started from the whole spread.
func StandardSpreadObjectsOf(org OrgState) []string {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		if canonical, ok := ResolveKnownStandardObjectName(name); ok && canonical == name && standardSpreadIncludes(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// StandardKeyPrefixOwner returns the catalog object whose standard key prefix
// is prefix, or "" when no catalog object carries it. Among several objects
// with the same catalog prefix the first by name wins, which is the object an
// org holding the whole spread reports for that prefix. Prefixes known only
// to the captured describe catalog are included; the reverse table is built
// on first use.
func StandardKeyPrefixOwner(prefix string) string {
	if owner := catalogKeyPrefixOwners()[prefix]; owner != "" {
		return owner
	}
	return standardKeyPrefixOwner(prefix)
}

var catalogKeyPrefixOwnerCache struct {
	once   sync.Once
	owners map[string]string
}

func catalogKeyPrefixOwners() map[string]string {
	catalogKeyPrefixOwnerCache.once.Do(func() {
		owners := make(map[string]string)
		for _, name := range KnownStandardObjectNames() {
			prefix := standardKeyPrefixForObject(name)
			if prefix == "" {
				continue
			}
			if _, taken := owners[prefix]; !taken {
				owners[prefix] = name
			}
		}
		catalogKeyPrefixOwnerCache.owners = owners
	})
	return catalogKeyPrefixOwnerCache.owners
}
