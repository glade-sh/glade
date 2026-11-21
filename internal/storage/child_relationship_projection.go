package storage

import "strings"

// ChildRelationshipTargetsComplete certifies that the full describe preparation
// cannot add a relationship target to the observed raw relation list. Mirror
// the exact feature-signature input of EnsureStandardObjectFields. Keep bare
// objects, Account's extra PersonAccounts overlay and all change events on the
// original path. Custom-setting removal can only delete a relation, and
// NameIndustriesChildRelationships can only rename its child side.
func ChildRelationshipTargetsComplete(d ObjectDefinition) bool {
	if len(d.Fields) == 0 || strings.EqualFold(d.APIName, "Account") || IsChangeEventObject(d) {
		return false
	}
	marker, applied := d.Metadata[standardFieldsOverlayMarker]
	if !applied {
		return false
	}
	return !standardObjectFieldsNeedWrite(d, canonicalFeatureSignature(strings.Split(marker, ",")))
}
