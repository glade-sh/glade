package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// REVERSE/TWO/FIELDS/ACCOUNT_TWO: viewer links inserted with omitted Visibility
// to Account or an ordinary custom record query back as AllUsers at 62/67/68.
// Explicit values/nulls, other shares, parents and insert failures keep their path.
func (e *Engine) applyContentChildLinkDefaults(objectName string, record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(objectName, "ContentDocumentLink") {
		return
	}
	if _, present := record.GetField("Visibility"); present || record.HasExplicitNull("Visibility") {
		return
	}
	share, present := record.GetField("ShareType")
	if !present || share.Kind != storage.ValueString || share.String != "V" {
		return
	}
	linked, _ := record.GetField("LinkedEntityId")
	document, _ := record.GetField("ContentDocumentId")
	parentID, documentID := idFromStorageValue(linked), idFromStorageValue(document)
	if parentID == "" || documentID == "" {
		return
	}
	_, doc, found := storage.LookupRecordByID(e.Org.Objects["ContentDocument"].Records, documentID)
	if !found || doc.System.IsDeleted {
		return
	}
	for _, object := range e.Org.Objects {
		definition := object.Definition
		if !strings.EqualFold(definition.APIName, "Account") &&
			(!strings.HasSuffix(strings.ToLower(definition.APIName), "__c") || storage.IsCustomSettingDefinition(definition)) {
			continue
		}
		_, parent, found := storage.LookupRecordByID(object.Records, parentID)
		if found && !parent.System.IsDeleted {
			record.Fields["Visibility"] = storage.StringValue("AllUsers")
			return
		}
	}
}
