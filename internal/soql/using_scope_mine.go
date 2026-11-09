package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ExecutionContext contains actor state that must not be stored in parsed query
// or schema caches. An omitted actor keeps owned-record Mine queries unsupported.
type ExecutionContext struct {
	APIVersion string
	Language   string
	// Locale is the user's LocaleSidKey; text comparison folds by it.
	Locale string
	// FormatDatetime renders a stored Datetime for FORMAT() in the user's
	// time zone and locale.
	FormatDatetime       func(text string) (string, bool)
	UserID               storage.ID
	FormulaUserID        storage.ID
	PermissionUserFields map[string]storage.Value
	// SetupUserID selects hierarchy settings without changing query-scope behavior.
	SetupUserID            storage.ID
	UserRecordAccess       func(objectName string, record storage.Record) storage.Record
	NestedUserRecordAccess func(objectName string, record storage.Record) storage.Record
	ChildRecordVisible     func(objectName string, record storage.Record) bool
	ChildFieldReadable     func(objectName, field string) bool
	WindowRecordVisible    func(objectName string, record storage.Record) bool
}

func (c ExecutionContext) formulaUserID() storage.ID {
	if c.FormulaUserID != "" {
		return c.FormulaUserID
	}
	return c.UserID
}

func (context ExecutionContext) formulaActor() storage.ID {
	if context.SetupUserID != "" {
		return context.SetupUserID
	}
	return context.UserID
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
