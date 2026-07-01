package dbmanager

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type Manager struct {
	Org *storage.OrgState
}

func New(org *storage.OrgState) Manager {
	return Manager{Org: org}
}

type ListObjectsOptions struct {
	Query string
}

type ObjectList struct {
	Objects []ObjectSummary `json:"objects"`
}

type ObjectSummary struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	PluralLabel string `json:"pluralLabel,omitempty"`
	KeyPrefix   string `json:"keyPrefix,omitempty"`
	Records     int    `json:"records"`
}

type ObjectDetail struct {
	Name        string        `json:"name"`
	Label       string        `json:"label"`
	PluralLabel string        `json:"pluralLabel,omitempty"`
	KeyPrefix   string        `json:"keyPrefix,omitempty"`
	Fields      []FieldEditor `json:"fields"`
}

type FieldEditor struct {
	Name           string           `json:"name"`
	Label          string           `json:"label"`
	Type           string           `json:"type"`
	DisplayType    string           `json:"displayType,omitempty"`
	Control        string           `json:"control"`
	Required       bool             `json:"required"`
	Createable     bool             `json:"createable"`
	Updateable     bool             `json:"updateable"`
	Nillable       bool             `json:"nillable"`
	ReferenceTo    []string         `json:"referenceTo,omitempty"`
	PicklistValues []PicklistChoice `json:"picklistValues,omitempty"`
	DefaultValue   *FieldInput      `json:"defaultValue,omitempty"`
}

type PicklistChoice struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Active  bool   `json:"active"`
	Default bool   `json:"default"`
}

type ListRecordsOptions struct {
	Query          string
	Limit          int
	Offset         int
	IncludeDeleted bool
}

type RecordList struct {
	Object  string      `json:"object"`
	Total   int         `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
	Records []RecordRow `json:"records"`
}

type RecordRow struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Title   string         `json:"title"`
	Deleted bool           `json:"deleted,omitempty"`
	Fields  map[string]any `json:"fields"`
}

func (m Manager) ListObjects(opts ListObjectsOptions) ObjectList {
	if m.Org == nil {
		return ObjectList{}
	}
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	names := make([]string, 0, len(m.Org.Objects))
	for name := range m.Org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	out := ObjectList{Objects: make([]ObjectSummary, 0, len(names))}
	for _, name := range names {
		object := m.Org.Objects[name]
		summary := ObjectSummary{
			Name:        firstNonEmpty(object.Definition.APIName, name),
			Label:       firstNonEmpty(object.Definition.Label, object.Definition.APIName, name),
			PluralLabel: object.Definition.PluralLabel,
			KeyPrefix:   object.Definition.KeyPrefix,
			Records:     liveRecordCount(object),
		}
		if query != "" && !objectSummaryMatches(summary, query) {
			continue
		}
		out.Objects = append(out.Objects, summary)
	}
	return out
}

func (m Manager) ObjectDetail(objectName string) (ObjectDetail, error) {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return ObjectDetail{}, fmt.Errorf("unknown object %s", objectName)
	}
	return ObjectDetail{
		Name:        firstNonEmpty(object.Definition.APIName, resolved),
		Label:       firstNonEmpty(object.Definition.Label, object.Definition.APIName, resolved),
		PluralLabel: object.Definition.PluralLabel,
		KeyPrefix:   object.Definition.KeyPrefix,
		Fields:      fieldEditors(object.Definition),
	}, nil
}

func (m Manager) ListRecords(objectName string, opts ListRecordsOptions) (RecordList, error) {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return RecordList{}, fmt.Errorf("unknown object %s", objectName)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	rows := make([]RecordRow, 0, len(object.Records))
	ids := sortedRecordIDs(object)
	for _, id := range ids {
		record := object.Records[id]
		record.ID = id
		record.Object = resolved
		if record.System.IsDeleted && !opts.IncludeDeleted {
			continue
		}
		row := recordRow(resolved, object.Definition, record)
		if query != "" && !rowMatches(row, query) {
			continue
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].Title)
		right := strings.ToLower(rows[j].Title)
		if left == right {
			return rows[i].ID < rows[j].ID
		}
		return left < right
	})
	total := len(rows)
	if offset > len(rows) {
		rows = nil
	} else {
		rows = rows[offset:]
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return RecordList{Object: resolved, Total: total, Limit: limit, Offset: offset, Records: rows}, nil
}

func (m Manager) RecordDetail(objectName, id string) (RecordRow, error) {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return RecordRow{}, fmt.Errorf("unknown object %s", objectName)
	}
	storedID, record, ok := storage.LookupRecordByID(object.Records, storage.ID(id))
	if !ok {
		return RecordRow{}, fmt.Errorf("record %s not found", id)
	}
	record.ID = storedID
	record.Object = resolved
	return recordRow(resolved, object.Definition, record), nil
}

func (m Manager) object(objectName string) (string, storage.ObjectState, bool) {
	if m.Org == nil {
		return "", storage.ObjectState{}, false
	}
	resolved, ok := storage.ResolveObjectName(*m.Org, objectName)
	if !ok {
		return "", storage.ObjectState{}, false
	}
	object, ok := m.Org.Objects[resolved]
	return resolved, object, ok
}

func liveRecordCount(object storage.ObjectState) int {
	count := 0
	for _, record := range object.Records {
		if !record.System.IsDeleted {
			count++
		}
	}
	return count
}

func fieldEditors(definition storage.ObjectDefinition) []FieldEditor {
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return fieldSortKey(names[i], definition.Fields[names[i]]) < fieldSortKey(names[j], definition.Fields[names[j]])
	})
	fields := make([]FieldEditor, 0, len(names))
	for _, name := range names {
		field := definition.Fields[name]
		editor := FieldEditor{
			Name:        firstNonEmpty(field.APIName, name),
			Label:       firstNonEmpty(field.Label, field.APIName, name),
			Type:        string(field.Type),
			DisplayType: field.DisplayType,
			Control:     fieldControl(name, field),
			Required:    fieldRequired(field),
			Nillable:    fieldNillable(field),
			ReferenceTo: append([]string(nil), field.ReferenceTo...),
		}
		editor.Createable = fieldCreateable(editor.Control, field)
		editor.Updateable = fieldUpdateable(editor.Control, field)
		for _, value := range field.PicklistValues {
			editor.PicklistValues = append(editor.PicklistValues, PicklistChoice{
				Value:   value.Value,
				Label:   firstNonEmpty(value.Label, value.Value),
				Active:  value.Active,
				Default: value.Default,
			})
		}
		if value, ok := storage.DefaultValueForRecordField(definition, storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}, field); ok {
			input := FieldInputFromStorageValue(field, value)
			editor.DefaultValue = &input
		}
		fields = append(fields, editor)
	}
	return fields
}

func fieldSortKey(name string, field storage.Field) string {
	apiName := firstNonEmpty(field.APIName, name)
	lower := strings.ToLower(apiName)
	switch {
	case strings.EqualFold(apiName, "Id"):
		return "00|" + lower
	case strings.EqualFold(apiName, "Name"):
		return "01|" + lower
	case fieldRequired(field):
		return "02|" + lower
	default:
		return "03|" + lower
	}
}

func fieldControl(name string, field storage.Field) string {
	if strings.EqualFold(firstNonEmpty(field.APIName, name), "Id") || field.AutoNumber || field.Type == storage.FieldCalculated || field.Type == storage.FieldSummary {
		return "readonly"
	}
	if field.Createable != nil && !*field.Createable && field.Updateable != nil && !*field.Updateable {
		return "readonly"
	}
	switch field.Type {
	case storage.FieldPicklist:
		return "picklist"
	case storage.FieldMultiPicklist:
		return "multipicklist"
	case storage.FieldReference:
		return "lookup"
	case storage.FieldBoolean:
		return "checkbox"
	case storage.FieldDate:
		return "date"
	case storage.FieldDateTime:
		return "datetime"
	case storage.FieldInteger, storage.FieldDecimal:
		return "number"
	case storage.FieldBlob:
		return "textarea"
	default:
		return "text"
	}
}

func fieldRequired(field storage.Field) bool {
	if field.Required {
		return true
	}
	return field.Nillable != nil && !*field.Nillable && !storage.FieldFlagValue(field.DefaultedOnCreate, false)
}

func fieldNillable(field storage.Field) bool {
	return storage.FieldFlagValue(field.Nillable, true)
}

func fieldCreateable(control string, field storage.Field) bool {
	if control == "readonly" {
		return false
	}
	return storage.FieldFlagValue(field.Createable, true)
}

func fieldUpdateable(control string, field storage.Field) bool {
	if control == "readonly" {
		return false
	}
	return storage.FieldFlagValue(field.Updateable, true)
}

func sortedRecordIDs(object storage.ObjectState) []storage.ID {
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]storage.ID, 0, len(ids))
	for _, id := range ids {
		out = append(out, storage.ID(id))
	}
	return out
}

func recordRow(objectName string, definition storage.ObjectDefinition, record storage.Record) RecordRow {
	row := RecordRow{
		ID:      string(record.ID),
		Object:  objectName,
		Title:   recordTitle(record),
		Deleted: record.System.IsDeleted,
		Fields:  make(map[string]any, len(definition.Fields)),
	}
	for name, field := range definition.Fields {
		apiName := firstNonEmpty(field.APIName, name)
		if record.HasExplicitNull(apiName) || record.HasExplicitNull(name) {
			row.Fields[apiName] = nil
			continue
		}
		if value, ok := record.GetField(apiName); ok {
			row.Fields[apiName] = StorageValueJSON(value)
			continue
		}
		if value, ok := record.GetField(name); ok {
			row.Fields[apiName] = StorageValueJSON(value)
		}
	}
	if row.Title == "" {
		row.Title = row.ID
	}
	return row
}

func recordTitle(record storage.Record) string {
	for _, field := range []string{"Name", "DeveloperName", "Username", "Subject"} {
		value, ok := record.GetField(field)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(storageValueString(value)); text != "" {
			return text
		}
	}
	return string(record.ID)
}

func rowMatches(row RecordRow, query string) bool {
	if strings.Contains(strings.ToLower(row.ID), query) || strings.Contains(strings.ToLower(row.Title), query) || strings.Contains(strings.ToLower(row.Object), query) {
		return true
	}
	for _, value := range row.Fields {
		if value == nil {
			continue
		}
		if strings.Contains(strings.ToLower(fmt.Sprint(value)), query) {
			return true
		}
	}
	return false
}

func objectSummaryMatches(summary ObjectSummary, query string) bool {
	for _, value := range []string{summary.Name, summary.Label, summary.PluralLabel} {
		lower := strings.ToLower(value)
		if strings.HasPrefix(lower, query) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
