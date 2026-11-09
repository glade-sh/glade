package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// portal_sharing_timing_c4 (API 62.0 and 67.0): inserting a User for a
// Contact with a Customer Community or Customer Community Login profile
// stores UserType CspLitePortal and IsPortalEnabled true at once; a Customer
// Community Plus profile stores PowerCustomerSuccess, gives the user a role
// with PortalType CustomerPortal and PortalRole PersonAccount, and writes an
// AccountShare on the Contact's Account (RowCause PortalImplicit, Read) for
// the user. Visibility is synchronous with the insert, in tests and outside.
func portalUserTypeCaptured(userType string) bool {
	return strings.EqualFold(userType, "CspLitePortal") || strings.EqualFold(userType, "PowerCustomerSuccess")
}

func (e *Engine) portalProfileUserType(record storage.Record) string {
	profileID := idFromStorageValue(record.Fields["ProfileId"])
	if e == nil || e.Org == nil || profileID == "" || idFromStorageValue(record.Fields["ContactId"]) == "" {
		return ""
	}
	_, profile, ok := storage.LookupRecordByID(e.Org.Objects["Profile"].Records, profileID)
	if !ok {
		return ""
	}
	userType := strings.TrimSpace(profile.Fields["UserType"].String)
	if !portalUserTypeCaptured(userType) {
		return ""
	}
	return userType
}

func (e *Engine) applyUserPortalDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || !strings.EqualFold(objectName, "User") {
		return
	}
	userType := e.portalProfileUserType(*record)
	if userType == "" {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if field, ok := storage.ResolveFieldName(definition, "", "UserType"); ok {
		record.Fields[field] = storage.StringValue(userType)
	}
	if field, ok := storage.ResolveFieldName(definition, "", "IsPortalEnabled"); ok {
		record.Fields[field] = storage.BooleanValue(true)
	}
	if !strings.EqualFold(userType, "PowerCustomerSuccess") || idFromStorageValue(record.Fields["UserRoleId"]) != "" {
		return
	}
	accountID := e.portalContactAccountID(*record)
	if accountID == "" {
		return
	}
	_, account, _ := storage.LookupRecordByID(e.Org.Objects["Account"].Records, accountID)
	roleID, ok := e.insertPortalSetupRecord("UserRole", map[string]storage.Value{
		// The role name is not captured; it follows the documented
		// "<Account> Customer User" portal role naming.
		"Name":            storage.StringValue(strings.TrimSpace(account.Fields["Name"].String + " Customer User")),
		"PortalType":      storage.StringValue("CustomerPortal"),
		"PortalRole":      storage.StringValue("PersonAccount"),
		"PortalAccountId": storage.IDValue(accountID),
	})
	if ok {
		deleteExplicitNull(record.ExplicitNulls, "UserRoleId")
		record.Fields["UserRoleId"] = storage.IDValue(roleID)
	}
}

func (e *Engine) portalContactAccountID(record storage.Record) storage.ID {
	contactID := idFromStorageValue(record.Fields["ContactId"])
	_, contact, ok := storage.LookupRecordByID(e.Org.Objects["Contact"].Records, contactID)
	if !ok {
		return ""
	}
	return idFromStorageValue(contact.Fields["AccountId"])
}

func (e *Engine) afterInsertPortalUser(record storage.Record) {
	if !strings.EqualFold(e.portalProfileUserType(record), "PowerCustomerSuccess") {
		return
	}
	accountID := e.portalContactAccountID(record)
	if accountID == "" {
		return
	}
	e.insertPortalSetupRecord("AccountShare", map[string]storage.Value{
		"AccountId":          storage.IDValue(accountID),
		"UserOrGroupId":      storage.IDValue(record.ID),
		"RowCause":           storage.StringValue("PortalImplicit"),
		"AccountAccessLevel": storage.StringValue("Read"),
	})
}

func (e *Engine) insertPortalSetupRecord(objectName string, fields map[string]storage.Value) (storage.ID, bool) {
	storage.EnsureStandardObject(e.Org, objectName)
	e.recordJournalSequence(objectName)
	id, err := e.IDs.Next(objectName)
	if err != nil {
		return "", false
	}
	storage.EnsureMutableObjectRecords(e.Org, objectName)
	state := e.Org.Objects[objectName]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordInsert(objectName, id)
	}
	state.Records[id] = storage.Record{ID: id, Object: objectName, Fields: fields}
	e.Org.Objects[objectName] = state
	e.RecordCreations.Record(id)
	return id, true
}
