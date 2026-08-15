package server

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func isDefaultValuesRoute(parts []string) bool {
	return len(parts) == 2 && parts[1] == "defaultValues"
}

func isQuickActionsRoute(parts []string) bool {
	if len(parts) == 2 {
		return parts[1] == "quickActions"
	}
	if len(parts) == 3 {
		return parts[1] == "quickActions"
	}
	return len(parts) == 4 && parts[1] == "quickActions" && parts[3] == "defaultValues"
}

func isListViewsRoute(parts []string) bool {
	return isListViewCollectionRoute(parts) || (len(parts) == 4 && parts[1] == "listviews" && (parts[3] == "describe" || parts[3] == "results"))
}

func isListViewCollectionRoute(parts []string) bool {
	return len(parts) == 2 && parts[1] == "listviews"
}

func isRowTemplatePlaceholder(id storage.ID) bool {
	text := string(id)
	return strings.Contains(text, "{") || strings.Contains(text, "}")
}

func isObjectMetadataRoute(parts []string) bool {
	if len(parts) == 2 {
		return parts[1] == "layouts" || parts[1] == "compactLayouts"
	}
	if len(parts) == 3 && parts[1] == "describe" {
		return parts[2] == "layouts" || parts[2] == "approvalLayouts" || parts[2] == "namedLayouts" || parts[2] == "compactLayouts"
	}
	if len(parts) == 4 && parts[1] == "describe" && parts[2] == "compactLayouts" {
		return parts[3] != ""
	}
	return len(parts) == 3 && parts[1] == "namedLayouts"
}

func isCompactLayoutsRoute(parts []string) bool {
	return (len(parts) == 2 && parts[1] == "compactLayouts") ||
		(len(parts) == 3 && parts[1] == "describe" && parts[2] == "compactLayouts") ||
		(len(parts) == 4 && parts[1] == "describe" && parts[2] == "compactLayouts" && parts[3] != "")
}

func describePayload(def storage.ObjectDefinition, org *storage.OrgState) map[string]any {
	fields := make([]map[string]any, 0, len(def.Fields))
	names := make([]string, 0, len(def.Fields))
	for name := range def.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		field := def.Fields[name]
		fields = append(fields, describeFieldPayload(field))
	}
	return map[string]any{
		"name":                def.APIName,
		"apiName":             def.APIName,
		"label":               labelOrFallback(def.Label, def.APIName),
		"labelPlural":         labelOrFallback(def.PluralLabel, labelOrFallback(def.Label, def.APIName)),
		"custom":              strings.HasSuffix(def.APIName, "__c") || strings.HasSuffix(def.APIName, "__mdt"),
		"keyPrefix":           def.KeyPrefix,
		"fields":              fields,
		"searchable":          def.EnableSearch,
		"queryable":           true,
		"createable":          true,
		"updateable":          true,
		"deletable":           true,
		"recordTypeInfos":     describeRecordTypeInfos(def.RecordTypes),
		"defaultRecordTypeId": defaultRecordTypeID(def.RecordTypes),
		"childRelationships":  describeChildRelationships(def.APIName, org),
	}
}

func describeFieldPayload(field storage.Field) map[string]any {
	createable := field.Type != storage.FieldID && field.Type != storage.FieldCalculated
	if field.Createable != nil {
		createable = *field.Createable
	}
	updateable := createable
	if field.Updateable != nil {
		updateable = *field.Updateable
	}
	referenceTo := append([]string(nil), field.ReferenceTo...)
	sort.Strings(referenceTo)
	nillable := !field.Required && field.Type != storage.FieldID && !strings.EqualFold(field.APIName, "Id")
	compoundFieldName := strings.TrimSpace(field.CompoundFieldName)
	return map[string]any{
		"name":                  field.APIName,
		"apiName":               field.APIName,
		"label":                 labelOrFallback(field.Label, field.APIName),
		"type":                  string(field.Type),
		"dataType":              lightningFieldDataType(field),
		"length":                describeFieldLength(field),
		"nillable":              nillable,
		"required":              !nillable,
		"accessible":            storage.FieldFlagValue(field.Accessible, true),
		"calculated":            field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != "",
		"compound":              describeFieldCompound(field),
		"compoundComponentName": describeCompoundComponentName(field, compoundFieldName),
		"compoundFieldName":     nullableString(compoundFieldName),
		"controllerName":        nullableString(field.PicklistController),
		"controllingFields":     append([]string(nil), field.FilteredLookupInfo.ControllingFields...),
		"createable":            createable,
		"custom":                strings.Contains(field.APIName, "__"),
		"extraTypeInfo":         describeFieldExtraTypeInfo(field),
		"filterable":            true,
		"filteredLookupInfo":    describeFilteredLookupInfo(field.FilteredLookupInfo),
		"htmlFormatted":         describeFieldHTMLFormatted(field),
		"inlineHelpText":        nullableString(field.InlineHelpText),
		"polymorphicForeignKey": len(referenceTo) > 1,
		"precision":             field.Precision,
		"reference":             field.Type == storage.FieldReference,
		"referenceTargetField":  nil,
		"scale":                 field.Scale,
		"searchPrefilterable":   false,
		"sortable":              field.Type != storage.FieldAddress && field.Type != storage.FieldLocation,
		"externalId":            field.ExternalID,
		"unique":                field.Unique,
		"idLookup":              field.Type == storage.FieldID || strings.EqualFold(field.APIName, "Id") || field.ExternalID,
		"referenceTo":           referenceTo,
		"referenceToInfos":      describeReferenceToInfos(referenceTo),
		"relationshipName":      nullableString(field.RelationshipName),
		"picklistValues":        describePicklistValues(field.PicklistValues),
		"nameField":             strings.EqualFold(field.APIName, "Name"),
		"updateable":            updateable,
	}
}

func describeReferenceToInfos(referenceTo []string) []map[string]any {
	out := make([]map[string]any, 0, len(referenceTo))
	for _, apiName := range referenceTo {
		apiName = strings.TrimSpace(apiName)
		if apiName == "" {
			continue
		}
		out = append(out, map[string]any{
			"apiName":    apiName,
			"name":       apiName,
			"nameFields": []string{"Name"},
		})
	}
	return out
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func describeFieldCompound(_ storage.Field) bool {
	return false
}

func describeCompoundComponentName(field storage.Field, compoundFieldName string) any {
	if compoundFieldName == "" {
		return nil
	}
	componentName := strings.TrimPrefix(field.APIName, compoundFieldName)
	componentName = strings.TrimPrefix(componentName, "_")
	if componentName == "" {
		return nil
	}
	return componentName
}

func describeFieldExtraTypeInfo(field storage.Field) any {
	displayType := strings.ToUpper(strings.TrimSpace(field.DisplayType))
	switch {
	case strings.Contains(displayType, "RICHTEXT"):
		return "RichTextArea"
	case strings.Contains(displayType, "TEXTAREA") || strings.Contains(displayType, "LONGTEXT"):
		return "PlainTextArea"
	default:
		return nil
	}
}

func describeFieldHTMLFormatted(field storage.Field) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(field.DisplayType)), "RICHTEXT")
}

func describeFilteredLookupInfo(info storage.FilteredLookupInfo) any {
	if len(info.ControllingFields) == 0 && !info.Dependent && !info.OptionalFilter {
		return nil
	}
	return map[string]any{
		"controllingFields": append([]string(nil), info.ControllingFields...),
		"dependent":         info.Dependent,
		"optionalFilter":    info.OptionalFilter,
	}
}

func lightningFieldDataType(field storage.Field) string {
	switch field.Type {
	case storage.FieldString:
		return "String"
	case storage.FieldPicklist:
		return "Picklist"
	case storage.FieldMultiPicklist:
		return "Multipicklist"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger:
		return "Int"
	case storage.FieldDecimal:
		return "Double"
	case storage.FieldDate:
		return "Date"
	case storage.FieldDateTime:
		return "DateTime"
	case storage.FieldReference, storage.FieldID:
		return "Reference"
	case storage.FieldCalculated:
		return "Calculated"
	default:
		if field.DisplayType != "" {
			return strings.Title(strings.ToLower(field.DisplayType))
		}
		return string(field.Type)
	}
}

func defaultRecordTypeID(recordTypes []storage.RecordTypeInfo) string {
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.ID != "" {
			return string(recordType.ID)
		}
	}
	for _, recordType := range recordTypes {
		if recordType.ID != "" && (recordType.Active || recordType.Available) {
			return string(recordType.ID)
		}
	}
	return "012000000000000AAA"
}

func describeFieldLength(field storage.Field) int {
	switch field.Type {
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist, storage.FieldID, storage.FieldReference:
		return 255
	case storage.FieldBlob:
		return 0
	default:
		return 0
	}
}

func describePicklistValues(values []storage.PicklistValue) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"value":        value.Value,
			"label":        labelOrFallback(value.Label, value.Value),
			"active":       value.Active,
			"defaultValue": value.Default,
		})
	}
	return out
}

func describeRecordTypeInfos(recordTypes []storage.RecordTypeInfo) []map[string]any {
	sorted := append([]storage.RecordTypeInfo(nil), recordTypes...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].DeveloperName == sorted[j].DeveloperName {
			if sorted[i].Name == sorted[j].Name {
				return sorted[i].ID < sorted[j].ID
			}
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].DeveloperName < sorted[j].DeveloperName
	})
	out := make([]map[string]any, 0, len(sorted))
	for _, recordType := range sorted {
		out = append(out, map[string]any{
			"recordTypeId":             recordType.ID.String(),
			"developerName":            recordType.DeveloperName,
			"name":                     labelOrFallback(recordType.Name, recordType.DeveloperName),
			"active":                   recordType.Active,
			"available":                recordType.Available,
			"defaultRecordTypeMapping": recordType.Default,
		})
	}
	return out
}

func describeChildRelationships(objectName string, org *storage.OrgState) []map[string]any {
	if org == nil {
		return []map[string]any{}
	}
	childNames := make([]string, 0, len(org.Objects))
	for childName := range org.Objects {
		childNames = append(childNames, childName)
	}
	sort.Strings(childNames)
	out := make([]map[string]any, 0)
	for _, childName := range childNames {
		relationships := append([]storage.Relationship(nil), org.Objects[childName].Definition.Relations...)
		sort.Slice(relationships, func(i, j int) bool {
			if relationships[i].ChildRelationship == relationships[j].ChildRelationship {
				return relationships[i].Field < relationships[j].Field
			}
			return relationships[i].ChildRelationship < relationships[j].ChildRelationship
		})
		for _, relationship := range relationships {
			if !relationshipTargetsObject(relationship, objectName) {
				continue
			}
			out = append(out, map[string]any{
				"cascadeDelete":       relationship.CascadeDelete,
				"childSObject":        childName,
				"deprecatedAndHidden": false,
				"field":               relationship.Field,
				"relationshipName":    relationship.ChildRelationship,
				"restrictedDelete":    relationship.RestrictedDelete,
			})
		}
	}
	return out
}

func relationshipTargetsObject(relationship storage.Relationship, objectName string) bool {
	for _, parent := range relationship.ParentObjects {
		if strings.EqualFold(parent, objectName) {
			return true
		}
	}
	return false
}

func labelOrFallback(label, fallback string) string {
	if label != "" {
		return label
	}
	return fallback
}

func listViewsPayload(def storage.ObjectDefinition, version string) map[string]any {
	return map[string]any{
		"done":       true,
		"size":       0,
		"listviews":  []map[string]any{},
		"objectType": def.APIName,
		"url":        "/services/data/" + version + "/sobjects/" + def.APIName + "/listviews",
		"message":    "List view metadata is not modeled; returning an empty local stub.",
	}
}

func (s *Server) listViewsPayload(def storage.ObjectDefinition, version string) map[string]any {
	views := s.Source.ListViews[def.APIName]
	if len(views) == 0 {
		return listViewsPayload(def, version)
	}
	items := make([]map[string]any, 0, len(views))
	for _, view := range views {
		base := "/services/data/" + version + "/sobjects/" + def.APIName + "/listviews/" + view.ID
		items = append(items, map[string]any{
			"id":            view.ID,
			"developerName": view.DeveloperName,
			"label":         view.Label,
			"describeUrl":   base + "/describe",
			"resultsUrl":    base + "/results",
			"url":           base,
		})
	}
	return map[string]any{
		"done":       true,
		"size":       len(items),
		"listviews":  items,
		"objectType": def.APIName,
		"url":        "/services/data/" + version + "/sobjects/" + def.APIName + "/listviews",
	}
}

func (s *Server) handleListViewMetadata(w http.ResponseWriter, object storage.ObjectState, version string, parts []string) {
	if len(parts) != 4 || parts[1] != "listviews" {
		writeSalesforceError(w, errUnsupportedFeature, "SObject list view describe and result execution are not modeled in the local server; collection discovery returns an empty local stub")
		return
	}
	if len(s.Source.ListViews[object.Definition.APIName]) == 0 {
		writeSalesforceError(w, errUnsupportedFeature, "SObject list view describe and result execution are not modeled in the local server; collection discovery returns an empty local stub")
		return
	}
	view, ok := s.findListView(object.Definition.APIName, parts[2])
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "list view not found")
		return
	}
	switch parts[3] {
	case "describe":
		writeJSON(w, http.StatusOK, s.listViewDescribePayload(object.Definition, version, view))
	case "results":
		writeJSON(w, http.StatusOK, s.listViewResultsPayload(object, version, view))
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) findListView(objectName, idOrName string) (listViewMetadata, bool) {
	for _, view := range s.Source.ListViews[objectName] {
		if view.ID == idOrName || strings.EqualFold(view.DeveloperName, idOrName) {
			return view, true
		}
	}
	return listViewMetadata{}, false
}

func (s *Server) listViewDescribePayload(def storage.ObjectDefinition, version string, view listViewMetadata) map[string]any {
	columns := view.Columns
	if len(columns) == 0 {
		columns = []string{"Id"}
		if _, ok := def.Fields["Name"]; ok {
			columns = append(columns, "Name")
		}
	}
	displayColumns := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		displayColumns = append(displayColumns, map[string]any{
			"fieldNameOrPath": column,
			"label":           column,
			"sortable":        true,
		})
	}
	query := "SELECT " + strings.Join(columns, ", ") + " FROM " + def.APIName
	base := "/services/data/" + version + "/sobjects/" + def.APIName + "/listviews/" + view.ID
	return map[string]any{
		"id":             view.ID,
		"developerName":  view.DeveloperName,
		"label":          view.Label,
		"sobjectType":    def.APIName,
		"columns":        columns,
		"displayColumns": displayColumns,
		"query":          query,
		"describeUrl":    base + "/describe",
		"resultsUrl":     base + "/results",
		"url":            base,
	}
}

func (s *Server) listViewResultsPayload(object storage.ObjectState, version string, view listViewMetadata) map[string]any {
	columns := view.Columns
	if len(columns) == 0 {
		columns = []string{"Id"}
		if _, ok := object.Definition.Fields["Name"]; ok {
			columns = append(columns, "Name")
		}
	}
	ids := make([]string, 0, len(object.Records))
	for id, record := range object.Records {
		if !record.System.IsDeleted {
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	rows := make([]map[string]any, 0, len(ids))
	for _, idText := range ids {
		record := object.Records[storage.ID(idText)]
		rows = append(rows, projectedRecordPayload(record, version, object.Definition.APIName, record.ID, columns))
	}
	return map[string]any{
		"id":         view.ID,
		"label":      view.Label,
		"size":       len(rows),
		"done":       true,
		"columns":    columns,
		"records":    rows,
		"query":      "SELECT " + strings.Join(columns, ", ") + " FROM " + object.Definition.APIName,
		"listViewId": view.ID,
	}
}

func compactLayoutsPayload(def storage.ObjectDefinition) map[string]any {
	return map[string]any{
		"compactLayouts":         []map[string]any{},
		"defaultCompactLayoutId": nil,
		"objectType":             def.APIName,
		"message":                "Compact layout metadata is not modeled; returning an empty local stub.",
	}
}

func (s *Server) compactLayoutsPayload(def storage.ObjectDefinition, parts []string) map[string]any {
	layouts := s.Source.Compact[def.APIName]
	if len(layouts) == 0 {
		return compactLayoutsPayload(def)
	}
	items := make([]map[string]any, 0, len(layouts))
	for _, layout := range layouts {
		items = append(items, map[string]any{
			"id":            layout.ID,
			"developerName": layout.DeveloperName,
			"label":         layout.Label,
			"fields":        layout.Fields,
			"objectType":    def.APIName,
		})
	}
	defaultID := layouts[0].ID
	payload := map[string]any{
		"compactLayouts":         items,
		"defaultCompactLayoutId": defaultID,
		"objectType":             def.APIName,
	}
	if len(parts) == 4 {
		for _, item := range items {
			if item["id"] == parts[3] || strings.EqualFold(fmt.Sprint(item["developerName"]), parts[3]) {
				return item
			}
		}
	}
	return payload
}

func (s *Server) hasLayoutMetadata(objectName string) bool {
	return len(s.Source.Layouts[objectName]) > 0
}

func (s *Server) layoutMetadataPayload(def storage.ObjectDefinition, version string, parts []string) map[string]any {
	layouts := s.Source.Layouts[def.APIName]
	items := make([]map[string]any, 0, len(layouts))
	for _, layout := range layouts {
		items = append(items, s.layoutMetadataItemPayload(def, version, layout))
	}
	if len(parts) == 3 && parts[1] == "namedLayouts" {
		for _, item := range items {
			if strings.EqualFold(fmt.Sprint(item["name"]), parts[2]) {
				return item
			}
		}
	}
	return map[string]any{
		"layouts":    items,
		"objectType": def.APIName,
		"url":        "/services/data/" + version + "/sobjects/" + def.APIName + "/describe/layouts",
	}
}

func (s *Server) layoutMetadataItemPayload(def storage.ObjectDefinition, version string, layout layoutMetadata) map[string]any {
	base := "/services/data/" + version + "/sobjects/" + def.APIName + "/namedLayouts/" + url.PathEscape(layout.Name)
	namespace := ""
	if s.Org != nil {
		namespace = s.Org.Namespace
	}
	item := map[string]any{
		"id":           layout.ID,
		"name":         layout.Name,
		"objectType":   def.APIName,
		"url":          base,
		"layoutType":   "Full",
		"mode":         "View",
		"recordTypeId": defaultRecordTypeID(def.RecordTypes),
		"sections":     []map[string]any{},
	}
	if len(layout.Sections) > 0 {
		item["sections"] = sourceLayoutSectionsPayload(layout, def, namespace)
	}
	return item
}

func objectResourcePayload(def storage.ObjectDefinition, version string) map[string]any {
	name := def.APIName
	label := def.Label
	if label == "" {
		label = name
	}
	base := "/services/data/" + version + "/sobjects/" + name
	describe := base + "/describe"
	recent := base + "/recent"
	return map[string]any{
		"name":           name,
		"label":          label,
		"keyPrefix":      def.KeyPrefix,
		"custom":         strings.HasSuffix(name, "__c"),
		"objectDescribe": describe,
		"recentItems":    recent,
		"describe":       describe,
		"url":            base,
		"urls": map[string]string{
			"rowTemplate":     base + "/{ID}",
			"defaultValues":   base + "/defaultValues?recordTypeId&fields",
			"describe":        describe,
			"recent":          recent,
			"updated":         base + "/updated",
			"deleted":         base + "/deleted",
			"items":           base,
			"layouts":         describe + "/layouts",
			"approvalLayouts": describe + "/approvalLayouts",
			"compactLayouts":  base + "/compactLayouts",
			"namedLayouts":    base + "/namedLayouts/{LAYOUT_NAME}",
			"quickActions":    base + "/quickActions",
			"listviews":       base + "/listviews",
		},
	}
}

func findExternalIDRecord(object storage.ObjectState, fieldName string, field storage.Field, value storage.Value) (storage.Record, storage.ID, int) {
	matches := make([]storage.ID, 0, 1)
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		storedValue, ok := record.Fields[fieldName]
		if !ok || !storageValuesEqual(field, storedValue, value) {
			continue
		}
		matches = append(matches, id)
	}
	if len(matches) != 1 {
		return storage.Record{}, "", len(matches)
	}
	id := matches[0]
	record := object.Records[id]
	if record.ID == "" {
		record.ID = id
	}
	if record.Object == "" {
		record.Object = object.Definition.APIName
	}
	return record, id, 1
}

func writeExternalIDLookupResult(w http.ResponseWriter, objectName, fieldName string, matches int) bool {
	switch matches {
	case 1:
		return true
	case 0:
		writeSalesforceError(w, errUnknownRecord)
	default:
		writeSalesforceError(w, errDuplicateValue, fmt.Sprintf("external id %s.%s matched multiple records", objectName, fieldName))
	}
	return false
}

func externalIDValueFromPath(field storage.Field, raw string) (storage.Value, error) {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return storage.IDValue(storage.ID(raw)), nil
	case storage.FieldInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid external id integer value %q", raw)
		}
		return storage.IntegerValue(value), nil
	case storage.FieldBoolean:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid external id boolean value %q", raw)
		}
		return storage.BooleanValue(value), nil
	case storage.FieldDecimal:
		return storage.DecimalValue(raw), nil
	case storage.FieldDate:
		return storage.DateValue(raw), nil
	case storage.FieldDateTime:
		return storage.DateTimeValue(raw), nil
	default:
		return storage.StringValue(raw), nil
	}
}

func storageValuesEqual(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			if !field.CaseSensitive {
				return strings.EqualFold(string(left.ID), right.String)
			}
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			if !field.CaseSensitive {
				return strings.EqualFold(left.String, string(right.ID))
			}
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return left.String == right.String
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func recordPayload(record storage.Record, version string, objectName string, id storage.ID) map[string]any {
	return recordPayloadWithProjection(record, version, objectName, id, nil, false)
}

func recordFieldProjectionFromRequest(w http.ResponseWriter, r *http.Request, definition storage.ObjectDefinition, namespace string) ([]string, bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("fields"))
	if raw == "" {
		return nil, false, true
	}
	fields := make([]string, 0)
	seen := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		requested := strings.TrimSpace(part)
		if requested == "" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, requested)
		if !ok {
			writeSalesforceError(w, errInvalidField, fmt.Sprintf("No such column %q on entity %q", requested, definition.APIName))
			return nil, false, false
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		fields = append(fields, canonical)
	}
	if len(fields) == 0 {
		return nil, false, true
	}
	return fields, true, true
}

func recordPayloadWithProjection(record storage.Record, version string, objectName string, id storage.ID, projection []string, projected bool) map[string]any {
	if record.Object != "" {
		objectName = record.Object
	}
	if record.ID != "" {
		id = record.ID
	}
	out := map[string]any{
		"attributes": map[string]any{
			"type": objectName,
			"url":  "/services/data/" + version + "/sobjects/" + objectName + "/" + string(id),
		},
		"Id": string(id),
	}
	if projected {
		for _, name := range projection {
			if name == "Id" {
				continue
			}
			if value, ok := record.Fields[name]; ok {
				out[name] = storageValueJSON(value)
				continue
			}
			out[name] = nil
		}
		return out
	}
	fieldNames := make([]string, 0, len(record.Fields)+len(record.ExplicitNulls))
	seen := make(map[string]bool, len(record.Fields)+len(record.ExplicitNulls))
	for name := range record.Fields {
		if name == "Id" || name == "attributes" {
			continue
		}
		fieldNames = append(fieldNames, name)
		seen[name] = true
	}
	for name, isNull := range record.ExplicitNulls {
		if !isNull || name == "Id" || name == "attributes" || seen[name] {
			continue
		}
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	for _, name := range fieldNames {
		if value, ok := record.Fields[name]; ok {
			out[name] = storageValueJSON(value)
			continue
		}
		out[name] = nil
	}
	return out
}

func compositeRetrieveFields(w http.ResponseWriter, definition storage.ObjectDefinition, namespace, rawFields string) ([]string, bool, bool) {
	if strings.TrimSpace(rawFields) == "" {
		return nil, false, true
	}
	parts := strings.Split(rawFields, ",")
	fields := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, raw := range parts {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok {
			writeSalesforceError(w, errInvalidField, fmt.Sprintf("unknown field %s.%s", definition.APIName, name))
			return nil, false, false
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		fields = append(fields, canonical)
	}
	if len(fields) == 0 {
		return nil, false, true
	}
	return fields, true, true
}

func projectedRecordPayload(record storage.Record, version string, objectName string, id storage.ID, fields []string) map[string]any {
	if record.Object != "" {
		objectName = record.Object
	}
	if record.ID != "" {
		id = record.ID
	}
	out := map[string]any{
		"attributes": map[string]any{
			"type": objectName,
			"url":  "/services/data/" + version + "/sobjects/" + objectName + "/" + string(id),
		},
		"Id": string(id),
	}
	for _, name := range fields {
		if name == "Id" {
			continue
		}
		if value, ok := record.Fields[name]; ok {
			out[name] = storageValueJSON(value)
			continue
		}
		out[name] = nil
	}
	return out
}

func storageValueJSON(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		items := make([]any, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, storageValueJSON(item))
		}
		return items
	default:
		return nil
	}
}

const (
	defaultRecentLimit = 25
	maxRecentLimit     = 200
)

func recentLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultRecentLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > maxRecentLimit {
		return 0, fmt.Errorf("limit must be a positive integer no greater than 200")
	}
	return limit, nil
}

func recentPayload(object storage.ObjectState, version string, limit int) []map[string]any {
	ids := make([]string, 0, len(object.Records))
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		ids = append(ids, string(id))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		name := id
		if value, ok := record.Fields["Name"]; ok {
			name = value.String
		}
		objectName := record.Object
		if objectName == "" {
			objectName = object.Definition.APIName
		}
		out = append(out, map[string]any{"Id": id, "Name": name, "attributes": map[string]any{"type": objectName, "url": "/services/data/" + version + "/sobjects/" + objectName + "/" + id}})
	}
	return out
}

type updatedResourcePayload struct {
	LatestDateCovered string   `json:"latestDateCovered"`
	IDs               []string `json:"ids"`
}

type deletedResourcePayload struct {
	EarliestDateAvailable string                 `json:"earliestDateAvailable"`
	LatestDateCovered     string                 `json:"latestDateCovered"`
	DeletedRecords        []deletedResourceEntry `json:"deletedRecords"`
}

type deletedResourceEntry struct {
	ID          string `json:"id"`
	DeletedDate string `json:"deletedDate"`
}

func updatedPayload(object storage.ObjectState, r *http.Request) (updatedResourcePayload, error) {
	bounds, err := queryDateBounds(r)
	if err != nil {
		return updatedResourcePayload{}, err
	}
	ids := make([]string, 0, len(object.Records))
	latest := ""
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		stamp := recordChangeTimestamp(record.System)
		if stamp == "" || !timestampInBounds(stamp, bounds) {
			continue
		}
		ids = append(ids, string(id))
		if compareTimestamps(stamp, latest) > 0 {
			latest = stamp
		}
	}
	sort.Strings(ids)
	return updatedResourcePayload{LatestDateCovered: latest, IDs: ids}, nil
}

func deletedPayload(object storage.ObjectState, r *http.Request) (deletedResourcePayload, error) {
	bounds, err := queryDateBounds(r)
	if err != nil {
		return deletedResourcePayload{}, err
	}
	ids := make([]string, 0, len(object.Records))
	deletedDates := make(map[string]string, len(object.Records))
	earliest := ""
	latest := ""
	for id, record := range object.Records {
		if !record.System.IsDeleted {
			continue
		}
		stamp := recordChangeTimestamp(record.System)
		if stamp == "" {
			continue
		}
		if earliest == "" || compareTimestamps(stamp, earliest) < 0 {
			earliest = stamp
		}
		if !timestampInBounds(stamp, bounds) {
			continue
		}
		stringID := string(id)
		ids = append(ids, stringID)
		deletedDates[stringID] = stamp
		if compareTimestamps(stamp, latest) > 0 {
			latest = stamp
		}
	}
	if earliest == "" && bounds.hasStart {
		earliest = bounds.start.UTC().Format(time.RFC3339)
	}
	sort.Strings(ids)
	records := make([]deletedResourceEntry, 0, len(ids))
	for _, id := range ids {
		records = append(records, deletedResourceEntry{ID: id, DeletedDate: deletedDates[id]})
	}
	return deletedResourcePayload{EarliestDateAvailable: earliest, LatestDateCovered: latest, DeletedRecords: records}, nil
}

type dateBounds struct {
	hasStart bool
	start    time.Time
	hasEnd   bool
	end      time.Time
}

func queryDateBounds(r *http.Request) (dateBounds, error) {
	query := r.URL.Query()
	var bounds dateBounds
	if raw := firstQueryValue(query, "start", "startDate"); raw != "" {
		start, err := parseRESTTimestamp(raw)
		if err != nil {
			return bounds, fmt.Errorf("malformed start date %q", raw)
		}
		bounds.hasStart = true
		bounds.start = start
	}
	if raw := firstQueryValue(query, "end", "endDate"); raw != "" {
		end, err := parseRESTTimestamp(raw)
		if err != nil {
			return bounds, fmt.Errorf("malformed end date %q", raw)
		}
		bounds.hasEnd = true
		bounds.end = end
	}
	return bounds, nil
}

func firstQueryValue(query map[string][]string, names ...string) string {
	for _, name := range names {
		for _, value := range query[name] {
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func recordChangeTimestamp(system storage.SystemFields) string {
	if system.LastModifiedDate != "" {
		return system.LastModifiedDate
	}
	if system.SystemModstamp != "" {
		return system.SystemModstamp
	}
	return system.CreatedDate
}

func timestampInBounds(stamp string, bounds dateBounds) bool {
	if !bounds.hasStart && !bounds.hasEnd {
		return true
	}
	parsed, err := parseRESTTimestamp(stamp)
	if err != nil {
		return false
	}
	if bounds.hasStart && parsed.Before(bounds.start) {
		return false
	}
	if bounds.hasEnd && !parsed.Before(bounds.end) {
		return false
	}
	return true
}

func compareTimestamps(left, right string) int {
	if right == "" {
		if left == "" {
			return 0
		}
		return 1
	}
	leftTime, leftErr := parseRESTTimestamp(left)
	rightTime, rightErr := parseRESTTimestamp(right)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftTime.Before(rightTime):
			return -1
		case leftTime.After(rightTime):
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(left, right)
}

func parseRESTTimestamp(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("malformed date")
}

func recentAllPayload(org storage.OrgState, version string, limit int) []map[string]any {
	out := make([]map[string]any, 0)
	for _, object := range org.Objects {
		out = append(out, recentPayload(object, version, limit)...)
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := out[i]["Id"].(string)
		right, _ := out[j]["Id"].(string)
		return left > right
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func toolingDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/tooling"
	return map[string]string{
		"completions":          base + "/completions",
		"executeAnonymous":     base + "/executeAnonymous",
		"query":                base + "/query",
		"queryAll":             base + "/queryAll",
		"runTestsAsynchronous": base + "/runTestsAsynchronous",
		"runTestsSynchronous":  base + "/runTestsSynchronous",
		"search":               base + "/search",
		"sobjects":             base + "/sobjects",
	}
}

func metadataRESTDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/metadata"
	return map[string]string{
		"components":       base + "/components",
		"deployRequest":    base + "/deployRequest",
		"describe":         base + "/describe",
		"describeMetadata": base + "/describeMetadata",
		"listMetadata":     base + "/listMetadata",
		"retrieveRequest":  base + "/retrieveRequest",
	}
}

func isMetadataReadDiscoveryRoute(name string) bool {
	switch name {
	case "components", "describe", "describeMetadata", "listMetadata":
		return true
	default:
		return false
	}
}

func metadataReadDiscoveryUnsupportedMessage(name string) string {
	switch name {
	case "components":
		return "Metadata REST component discovery is not implemented in the local server; use source files and glade inspect/check for local metadata state"
	case "describe", "describeMetadata":
		return "Metadata REST describeMetadata is not implemented in the local server; use SObject describe and project metadata files for local shape information"
	case "listMetadata":
		return "Metadata REST listMetadata is not implemented in the local server; use source file discovery for local metadata listings"
	default:
		return "Metadata REST read/discovery is not implemented in the local server"
	}
}

func bulkJobsDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/jobs"
	return map[string]string{
		"query":  base + "/query",
		"ingest": base + "/ingest",
	}
}

func resourceDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version
	return map[string]string{
		"actions":      base + "/actions",
		"analytics":    base + "/analytics",
		"appMenu":      base + "/appMenu",
		"apps":         base + "/apps",
		"chatter":      base + "/chatter",
		"composite":    base + "/composite",
		"connect":      base + "/connect",
		"jobs":         base + "/jobs",
		"limits":       base + "/limits",
		"metadata":     base + "/metadata",
		"glade":        base + "/glade",
		"process":      base + "/process",
		"query":        base + "/query",
		"queryAll":     base + "/queryAll",
		"quickActions": base + "/quickActions",
		"recent":       base + "/recent",
		"search":       base + "/search",
		"sobjects":     base + "/sobjects",
		"support":      base + "/support",
		"tabs":         base + "/tabs",
		"theme":        base + "/theme",
		"tooling":      base + "/tooling",
		"wave":         base + "/wave",
	}
}

func (s *Server) apiVersionDiscoveryPayload() []apiVersionEntry {
	adv := s.advertisedRESTAPIVersion()
	return []apiVersionEntry{{
		Version: adv,
		Label:   "GLADE Local API v" + adv,
		URL:     "/services/data/v" + adv,
	}}
}

func unsupportedRESTNamespaceMessage(namespace string) (string, bool) {
	display, ok := unsupportedRESTNamespaces[namespace]
	if !ok {
		return "", false
	}
	return display + " REST namespace is not implemented in the local server", true
}

func compositeResults(results []dml.Result, referenceIDs []string) []map[string]any {
	return compositeResultRows(results, referenceIDs, false)
}

func compositeUpsertResults(results []dml.Result, referenceIDs []string) []map[string]any {
	return compositeResultRows(results, referenceIDs, true)
}

func compositeAllOrNoneRollbackResults(results []dml.Result) []dml.Result {
	out := make([]dml.Result, len(results))
	copy(out, results)
	for i, result := range out {
		if !result.Success {
			continue
		}
		out[i].Success = false
		out[i].StatusCode = "ALL_OR_NONE_OPERATION_ROLLED_BACK"
		out[i].Error = "dml: operation rolled back because allOrNone request failed"
		out[i].Errors = []dml.Error{{
			StatusCode: "ALL_OR_NONE_OPERATION_ROLLED_BACK",
			Message:    "dml: operation rolled back because allOrNone request failed",
			Fields:     []string{},
		}}
	}
	return out
}

func compositeResultRows(results []dml.Result, referenceIDs []string, includeCreated bool) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for i, result := range results {
		row := map[string]any{"id": result.ID, "success": result.Success, "errors": []map[string]any{}}
		if includeCreated {
			row["created"] = result.Created
		}
		if i < len(referenceIDs) && referenceIDs[i] != "" {
			row["referenceId"] = referenceIDs[i]
		}
		if !result.Success {
			row["errors"] = compositeErrorRows(result)
		}
		out = append(out, row)
	}
	return out
}

func compositeErrorRows(result dml.Result) []map[string]any {
	if len(result.Errors) == 0 {
		statusCode := result.StatusCode
		if statusCode == "" {
			statusCode = salesforceErrorCode(errDMLFailure)
		}
		return []map[string]any{{"statusCode": statusCode, "message": result.Error, "fields": result.Fields}}
	}
	errors := make([]map[string]any, 0, len(result.Errors))
	for _, err := range result.Errors {
		statusCode := err.StatusCode
		if statusCode == "" {
			statusCode = salesforceErrorCode(errDMLFailure)
		}
		errors = append(errors, map[string]any{"statusCode": statusCode, "message": err.Message, "fields": err.Fields})
	}
	return errors
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	out := raw[:0]
	for _, part := range raw {
		if part != "" {
			unescaped, err := url.PathUnescape(part)
			if err != nil {
				unescaped = part
			}
			out = append(out, unescaped)
		}
	}
	return out
}

func URL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return fmt.Sprintf("http://%s", addr)
}
