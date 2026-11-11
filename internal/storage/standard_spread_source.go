package storage

import (
	"slices"
	"sync"
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
	canonical, ok := spreadObjectOwed(org, name)
	if !ok {
		return "", false
	}
	template, ok := spreadTemplate(org, canonical)
	if !ok {
		return "", false
	}
	org.Objects[canonical] = template.CloneRuntimeFrozenDefinition()
	return canonical, true
}

// materializeSpreadSource installs every owed object, then marks the org
// complete. It never clears the stamp.
func materializeSpreadSource(org *OrgState) bool {
	if org == nil || org.StandardSpread == "" {
		return false
	}
	for _, name := range KnownStandardObjectNames() {
		if _, owed := spreadObjectOwed(org, name); !owed {
			continue
		}
		if _, ok := installSpreadObject(org, name); !ok {
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
