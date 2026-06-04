package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) applyFileInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || !strings.EqualFold(objectName, "Document") {
		return
	}
	field, ok := definition.Fields["FolderId"]
	if !ok || !fieldReferencesObject(field, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := record.Fields["FolderId"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["FolderId"] {
		return
	}
	record.Fields["FolderId"] = storage.IDValue(e.systemUserID())
}

func fieldReferencesObject(field storage.Field, objectName string) bool {
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, objectName) {
			return true
		}
	}
	return false
}

func (e *Engine) afterInsertContentVersion(version storage.Record) error {
	contentDocumentID := idFromStorageValue(version.Fields["ContentDocumentId"])
	contentDocumentWasCreated := contentDocumentID == ""
	if contentDocumentID == "" {
		document := storage.Record{
			Object: "ContentDocument",
			Fields: map[string]storage.Value{
				"Title":                    version.Fields["Title"].Clone(),
				"LatestPublishedVersionId": storage.IDValue(version.ID),
			},
		}
		if size, ok := e.contentDocumentSize(version); ok {
			document.Fields["ContentSize"] = storage.IntegerValue(size)
			document.Fields["ContentSizeLong"] = storage.IntegerValue(size)
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		id, err := e.insertPlatformRecord(document)
		if err != nil {
			return err
		}
		contentDocumentID = id
		storage.EnsureMutableObjectRecords(e.Org, "ContentVersion")
		contentVersionObject := e.Org.Objects["ContentVersion"]
		stored := contentVersionObject.Records[version.ID]
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate("ContentVersion", version.ID, stored)
		}
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["ContentDocumentId"] = storage.IDValue(contentDocumentID)
		contentVersionObject.Records[version.ID] = stored
		e.Org.Objects["ContentVersion"] = contentVersionObject
	} else {
		storage.EnsureMutableObjectRecords(e.Org, "ContentDocument")
		contentDocumentObject := e.Org.Objects["ContentDocument"]
		document, exists := contentDocumentObject.Records[contentDocumentID]
		if !exists {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{"ContentDocumentId"}, "dml: ContentDocument %s does not exist", contentDocumentID)
		}
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate("ContentDocument", contentDocumentID, document)
		}
		if document.Fields == nil {
			document.Fields = make(map[string]storage.Value)
		}
		document.Fields["LatestPublishedVersionId"] = storage.IDValue(version.ID)
		if title, ok := version.Fields["Title"]; ok {
			document.Fields["Title"] = title.Clone()
		}
		if size, ok := e.contentDocumentSize(version); ok {
			document.Fields["ContentSize"] = storage.IntegerValue(size)
			document.Fields["ContentSizeLong"] = storage.IntegerValue(size)
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		contentDocumentObject.Records[contentDocumentID] = document
		e.Org.Objects["ContentDocument"] = contentDocumentObject
	}
	e.markLatestContentVersion(contentDocumentID, version.ID)
	locationID := idFromStorageValue(version.Fields["FirstPublishLocationId"])
	if locationID == "" && contentDocumentWasCreated {
		if version.System.OwnerID != "" {
			locationID = version.System.OwnerID
		} else if version.System.CreatedByID != "" {
			locationID = version.System.CreatedByID
		} else {
			locationID = e.systemUserID()
		}
	}
	if locationID != "" {
		link := storage.Record{
			Object: "ContentDocumentLink",
			Fields: map[string]storage.Value{
				"ContentDocumentId": storage.IDValue(contentDocumentID),
				"LinkedEntityId":    storage.IDValue(locationID),
				"ShareType":         storage.StringValue("V"),
				"Visibility":        storage.StringValue("AllUsers"),
			},
		}
		if _, err := e.insertOne(link, nil); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) afterInsertContentDistribution(id storage.ID) {
	storage.EnsureMutableObjectRecords(e.Org, "ContentDistribution")
	object := e.Org.Objects["ContentDistribution"]
	record, ok := object.Records[id]
	if !ok {
		return
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate("ContentDistribution", id, record)
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	base := "https://glade.local/content/" + string(id)
	if _, ok := record.Fields["ContentDownloadUrl"]; !ok {
		record.Fields["ContentDownloadUrl"] = storage.StringValue(base + "/download")
	}
	if _, ok := record.Fields["DistributionPublicUrl"]; !ok {
		record.Fields["DistributionPublicUrl"] = storage.StringValue(base)
	}
	object.Records[id] = record
	e.Org.Objects["ContentDistribution"] = object
}

func (e *Engine) afterInsertEmailMessage(message storage.Record) error {
	toIDs, ok := message.GetField("ToIds")
	if !ok || toIDs.Kind != storage.ValueList {
		return nil
	}
	for _, toID := range toIDs.List {
		relationID := valueAsIDString(toID)
		if relationID == "" {
			continue
		}
		if relationID == "system" {
			relationID = string(e.systemUserID())
		}
		if err := storage.ValidateID(storage.ID(relationID)); err != nil {
			continue
		}
		storage.EnsureStandardObject(e.Org, "EmailMessageRelation")
		relation := storage.Record{
			Object: "EmailMessageRelation",
			Fields: map[string]storage.Value{
				"EmailMessageId": storage.IDValue(message.ID),
				"RelationId":     storage.IDValue(storage.ID(relationID)),
				"RelationType":   storage.StringValue("ToAddress"),
			},
		}
		if toAddress, ok := message.GetField("ToAddress"); ok && toAddress.String != "" {
			relation.Fields["RelationAddress"] = storage.StringValue(toAddress.String)
		}
		if _, err := e.insertPlatformRecord(relation); err != nil {
			return err
		}
	}
	return nil
}

func valueAsIDString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return strings.TrimSpace(value.String)
	default:
		return ""
	}
}

func (e *Engine) contentDocumentSize(version storage.Record) (int64, bool) {
	documentObject, ok := e.Org.Objects["ContentDocument"]
	if !ok {
		return 0, false
	}
	if _, ok := documentObject.Definition.Fields["ContentSize"]; !ok {
		return 0, false
	}
	data, ok := version.Fields["VersionData"]
	if !ok {
		return 0, false
	}
	switch data.Kind {
	case storage.ValueBlob, storage.ValueString:
		return int64(len(data.String)), true
	default:
		return 0, false
	}
}

func (e *Engine) markLatestContentVersion(contentDocumentID storage.ID, latestVersionID storage.ID) {
	storage.EnsureMutableObjectRecords(e.Org, "ContentVersion")
	contentVersionObject := e.Org.Objects["ContentVersion"]
	changed := false
	for id, stored := range contentVersionObject.Records {
		if idFromStorageValue(stored.Fields["ContentDocumentId"]) != contentDocumentID {
			continue
		}
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate("ContentVersion", id, stored)
		}
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["IsLatest"] = storage.BooleanValue(id == latestVersionID)
		contentVersionObject.Records[id] = stored
		changed = true
	}
	if changed {
		e.Org.Objects["ContentVersion"] = contentVersionObject
	}
}

func fileExtension(path string) string {
	lastSlash := strings.LastIndexAny(path, `/\`)
	lastDot := strings.LastIndex(path, ".")
	if lastDot <= lastSlash || lastDot == len(path)-1 {
		return ""
	}
	return path[lastDot+1:]
}

func contentDocumentFileType(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "docx":
		return "WORD_X"
	case "xlsx":
		return "EXCEL_X"
	case "pptx":
		return "POWER_POINT_X"
	case "pdf":
		return "PDF"
	case "jpg", "jpeg", "gif", "png":
		return strings.ToUpper(extension)
	case "m4a":
		return "M4A"
	default:
		return strings.ToUpper(extension)
	}
}
