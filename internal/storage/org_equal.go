package storage

import "reflect"

// PersistedFieldsIdentical reports whether two org states are deeply equal in
// every field that JSON serialization includes. It is a conservative fast
// path: true guarantees the serialized forms are byte-identical, while false
// only means the states were not provably identical here, so callers that need
// an exact answer must fall back to comparing serialized forms. Runtime-only
// fields (clocks, caches, generation counters) are ignored.
func (o OrgState) PersistedFieldsIdentical(other OrgState) bool {
	return reflect.DeepEqual(o.persistedView(), other.persistedView())
}

// persistedView copies the org with every runtime-only field cleared. A field
// added to OrgState later and missed here keeps its value, which can only make
// the comparison report false.
func (o OrgState) persistedView() OrgState {
	o.ChangeDataCaptureTestSetup = nil
	o.formulaUserID = ""
	o.formulaPermissionFields = nil
	o.Now = nil
	o.Today = nil
	o.PlatformCache = nil
	o.PlatformCacheTestSetup = nil
	o.SystemTimestampBase = ""
	o.SystemTimestampSequence = 0
	o.RuntimeSchemaStamp = ""
	o.objectNameCache = nil
	o.keyPrefixesValidated = false
	o.lazyStandardSpread = false
	o.spreadPrefixSeed = nil
	o.keyPrefixesValidatedObjectCount = 0
	o.keyPrefixesValidatedPrefixes = nil
	o.keyPrefixesValidatedGeneration = 0
	o.schemaGeneration = 0
	o.runtimeGeneration = nil
	// Clone turns an empty slice into nil; omitempty serializes both alike.
	if len(o.Metadata.FeatureParameters) == 0 {
		o.Metadata.FeatureParameters = nil
	}
	return o
}
