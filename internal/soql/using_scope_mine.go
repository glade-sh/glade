package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ExecutionContext contains actor state that must not be stored in parsed query
// or schema caches. An omitted actor keeps owned-record Mine queries unsupported.
type ExecutionContext struct {
	UserID           storage.ID
	UserRecordAccess func(objectName string, record storage.Record) storage.Record
}

func usingScopeOwnerFilter(definition storage.ObjectDefinition, scope string, context ExecutionContext) (string, storage.ID, error) {
	if scope == "" || strings.EqualFold(scope, "everything") {
		return "", "", nil
	}
	unsupported := func() (string, storage.ID, error) {
		return "", "", unsupportedSOQLErrorf("USING SCOPE %s is not supported by the local SOQL runtime", scope)
	}
	if !strings.EqualFold(scope, "mine") {
		return unsupported()
	}
	ownerField, found := storage.ResolveFieldName(definition, "", "OwnerId")
	if !found {
		// Owned observations cover User's ownerless scope independently of
		// owned objects; other ownerless object kinds remain unobserved.
		if strings.EqualFold(definition.APIName, "User") {
			return "", "", nil
		}
		return unsupported()
	}
	field := definition.Fields[ownerField]
	if field.Type != storage.FieldReference || context.UserID == "" {
		return unsupported()
	}
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, "User") {
			return ownerField, context.UserID, nil
		}
	}
	return unsupported()
}
