package dml

import "github.com/glade-sh/glade/internal/storage"

// Custom managed shares have insertion constraints beyond ordinary references
// and restricted picklists. Keep this on the shared DML path for every caller.
func (e *Engine) validateManagedShareInsert(record storage.Record) error {
	if e.Org == nil {
		return nil
	}
	definition := e.Org.Objects[record.Object].Definition
	parentName := definition.Metadata["parentObject"]
	if definition.Metadata["kind"] != "generatedShare" || parentName == "" {
		return nil
	}
	access, hasAccess := record.GetField("AccessLevel")
	if hasAccess && access.Kind == storage.ValueString && access.String != "Read" && access.String != "Edit" && access.String != "All" {
		return managedShareStatusError("INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST")
	}
	parent, hasParent := record.GetField("ParentId")
	parentID := idFromStorageValue(parent)
	if !hasParent || parent.Kind == storage.ValueNull || parentID == "" {
		return managedShareStatusError("FIELD_INTEGRITY_EXCEPTION")
	}
	user, hasUser := record.GetField("UserOrGroupId")
	userID := idFromStorageValue(user)
	if !hasUser || user.Kind == storage.ValueNull || userID == "" {
		return managedShareStatusError("REQUIRED_FIELD_MISSING")
	}
	if !hasAccess || access.Kind == storage.ValueNull || access.String == "" {
		return managedShareStatusError("REQUIRED_FIELD_MISSING")
	}
	reason, _ := record.GetField("RowCause")
	if reason.String == "Owner" {
		return managedShareStatusError("FIELD_INTEGRITY_EXCEPTION")
	}
	if access.String == "All" {
		return managedShareStatusError("INVALID_ACCESS_LEVEL")
	}
	if canonical, ok := storage.ResolveObjectName(*e.Org, parentName); ok && (reason.String == "" || reason.String == "Manual") {
		_, parentRecord, found := storage.LookupRecordByID(e.Org.Objects[canonical].Records, parentID)
		if found && storage.IDsEqual(parentRecord.System.OwnerID, userID) {
			return managedShareStatusError("INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY")
		}
	}
	return nil
}

// The native managed-share rows capture status codes only. Use the captured
// code as the local failure message; native message and field payloads are
// unspecified. A nonempty message preserves the existing DML failure path.
func managedShareStatusError(code string) error {
	return dmlErrorf(code, nil, "%s", code)
}
