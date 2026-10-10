package server

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// UI API labels differ from the underlying field labels. These standard label
// aliases are captured in full and compact layouts at APIs 59 and 67.
// Project layout structure and permissions still come from source and schema.
var standardLayoutItemLabels = map[string]map[string]string{
	"Account":     {"OwnerId": "Account Owner", "Rating": "Rating", "Phone": "Phone", "ParentId": "Parent Account", "Fax": "Fax", "Type": "Type", "BillingAddress": "Billing Address", "ShippingAddress": "Shipping Address", "CreatedById": "Created By", "LastModifiedById": "Last Modified By", "Description": "Description"},
	"Contact":     {"OwnerId": "Contact Owner", "Phone": "Phone", "Name": "Name", "AccountId": "Account Name", "MobilePhone": "Mobile", "Fax": "Fax", "ReportsToId": "Reports To", "AssistantName": "Assistant", "MailingAddress": "Mailing Address", "OtherAddress": "Other Address", "CreatedById": "Created By", "LastModifiedById": "Last Modified By", "Description": "Description"},
	"Opportunity": {"OwnerId": "Opportunity Owner", "ExpectedRevenue": "Expected Revenue", "Name": "Opportunity Name", "AccountId": "Account Name", "Type": "Type", "CampaignId": "Primary Campaign Source", "CreatedById": "Created By", "LastModifiedById": "Last Modified By"},
}

// r_layout_Account_Full_Create and r_layout_Contact_Full_View capture these
// displayed components and their order at APIs 59/67. This presentation list
// does not establish membership: every component must belong to the compound
// in schema metadata. Code and geolocation members are absent from these rows.
var standardAddressLayoutPresentation = map[string]map[string][]string{
	"Account": {
		"BillingAddress":  {"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry"},
		"ShippingAddress": {"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry"},
	},
	"Contact": {
		"MailingAddress": {"MailingStreet", "MailingCity", "MailingState", "MailingPostalCode", "MailingCountry"},
		"OtherAddress":   {"OtherStreet", "OtherCity", "OtherState", "OtherPostalCode", "OtherCountry"},
	},
}

func objectMetadataRecordTypeID(def storage.ObjectDefinition, requested string) string {
	id := strings.TrimSpace(requested)
	if strings.HasPrefix(id, "012") && (len(id) == 15 || len(id) == 18) {
		return id
	}
	return createDefaultsRecordTypeID(def, "")
}

func objectMetadataLayout(objectName string, def storage.ObjectDefinition, namespace, recordTypeID, layoutType, mode string, source SourceMetadata) (map[string]any, bool) {
	var sections []map[string]any
	var id string
	if layoutType == "Compact" {
		layouts := source.Compact[objectName]
		if len(layouts) == 0 {
			return nil, false
		}
		id = layouts[0].ID
		rows := []map[string]any{}
		for _, name := range layouts[0].Fields {
			item, ok := objectMetadataLayoutItem(def, namespace, layoutItemMetadata{Field: name}, "Compact", mode)
			if ok {
				rows = append(rows, map[string]any{"layoutItems": []map[string]any{item}})
			}
		}
		sections = []map[string]any{{"heading": nil, "columns": 1, "rows": len(rows), "layoutRows": rows, "collapsible": false, "useHeading": false, "tabOrder": nil}}
	} else {
		layout, ok := sourceCreateLayout(source, objectName)
		if !ok {
			return nil, false
		}
		id = layout.ID
		sections = objectMetadataLayoutSections(layout, def, namespace, mode)
	}
	return map[string]any{"id": id, "objectApiName": objectName, "recordTypeId": recordTypeID, "layoutType": layoutType, "mode": mode, "saveOptions": []map[string]any{}, "sections": sections}, true
}

func objectMetadataLayoutSections(layout layoutMetadata, def storage.ObjectDefinition, namespace, mode string) []map[string]any {
	sections := []map[string]any{}
	for _, section := range layout.Sections {
		columns := make([][]map[string]any, len(section.Columns))
		maxRows := 0
		for col, column := range section.Columns {
			for _, sourceItem := range column.Items {
				item, ok := objectMetadataLayoutItem(def, namespace, sourceItem, "Full", mode)
				if ok {
					columns[col] = append(columns[col], item)
				}
			}
			if len(columns[col]) > maxRows {
				maxRows = len(columns[col])
			}
		}
		if maxRows == 0 {
			continue
		}
		rows := make([]map[string]any, 0, maxRows)
		for row := 0; row < maxRows; row++ {
			items := []map[string]any{}
			for _, column := range columns {
				if row < len(column) {
					items = append(items, column[row])
				}
			}
			rows = append(rows, map[string]any{"layoutItems": items})
		}
		heading := section.DetailHeading
		if mode == "Create" {
			heading = true
		} else if mode == "Edit" {
			heading = section.EditHeading
		}
		sections = append(sections, map[string]any{"id": section.ID, "heading": section.Label, "columns": len(columns), "rows": len(rows), "layoutRows": rows, "collapsible": false, "useHeading": heading, "tabOrder": layoutSectionTabOrder(section.Style)})
	}
	return sections
}

func objectMetadataLayoutItem(def storage.ObjectDefinition, namespace string, item layoutItemMetadata, layoutType, mode string) (map[string]any, bool) {
	if item.EmptySpace {
		if mode == "Create" {
			return nil, false
		}
		return map[string]any{"label": "", "lookupIdApiName": nil, "required": false, "editableForNew": false, "editableForUpdate": false, "sortable": false, "uiBehavior": nil, "layoutComponents": []map[string]any{{"componentType": "EmptySpace", "apiName": nil}}}, true
	}
	name, ok := storage.ResolveFieldName(def, namespace, item.Field)
	if !ok {
		return nil, false
	}
	field := def.Fields[name]
	readonly := strings.EqualFold(item.Behavior, "Readonly") || field.Type == storage.FieldCalculated || field.Formula != ""
	if layoutType == "Full" && mode == "Create" && readonly {
		return nil, false
	}
	label := labelOrFallback(field.Label, name)
	if alias := standardLayoutItemLabels[def.APIName][name]; alias != "" {
		label = alias
	}
	components := []map[string]any{}
	componentNames := objectMetadataLayoutComponentNames(def, name)
	for _, componentName := range componentNames {
		component, exists := def.Fields[componentName]
		if !exists {
			continue
		}
		components = append(components, map[string]any{"apiName": componentName, "componentType": "Field", "label": labelOrFallback(component.Label, componentName)})
	}
	if len(components) == 0 {
		return nil, false
	}
	var lookup any
	if field.Type == storage.FieldReference {
		lookup = name
	}
	if name == "Name" {
		lookup = "Id"
	}
	editableNew, editableUpdate := fieldCreateable(field), fieldUpdateable(field)
	if len(componentNames) > 1 {
		editableNew, editableUpdate = true, true
		for _, componentName := range componentNames {
			component, exists := def.Fields[componentName]
			editableNew = editableNew && exists && fieldCreateable(component)
			editableUpdate = editableUpdate && exists && fieldUpdateable(component)
		}
	}
	standardOwner := name == "OwnerId" && standardLayoutItemLabels[def.APIName] != nil
	if readonly || standardOwner {
		editableNew, editableUpdate = false, false
	}
	if layoutType == "Full" && mode == "Create" && !editableNew && !standardOwner {
		return nil, false
	}
	var behavior any
	required := false
	if layoutType == "Full" {
		required = strings.EqualFold(item.Behavior, "Required")
		behavior = "Edit"
		if readonly {
			behavior = "Readonly"
		} else if required {
			behavior = "Required"
		}
	}
	return map[string]any{"label": label, "lookupIdApiName": lookup, "required": required, "editableForNew": editableNew, "editableForUpdate": editableUpdate, "sortable": false, "uiBehavior": behavior, "layoutComponents": components}, true
}

func objectMetadataLayoutComponentNames(def storage.ObjectDefinition, name string) []string {
	field := def.Fields[name]
	if name == "Name" {
		// r_layout_Contact_Full_View captures this presentation order. Each
		// component's own schema binding establishes its membership.
		components := []string{}
		for _, componentName := range []string{"Salutation", "FirstName", "LastName"} {
			if def.Fields[componentName].CompoundFieldName == name {
				components = append(components, componentName)
			}
		}
		if len(components) > 0 {
			return components
		}
	}
	if field.Type == storage.FieldAddress {
		members := map[string]bool{}
		for componentName, component := range def.Fields {
			if componentName != name && component.CompoundFieldName == name {
				members[componentName] = true
			}
		}
		if presentation, captured := standardAddressLayoutPresentation[def.APIName][name]; captured {
			components := []string{}
			for _, componentName := range presentation {
				if members[componentName] {
					components = append(components, componentName)
				}
			}
			return components
		}
		// Without a captured presentation, retain only actual schema members
		// in deterministic order; no field names are synthesized.
		if len(members) > 0 {
			components := make([]string, 0, len(members))
			for componentName := range members {
				components = append(components, componentName)
			}
			sort.Strings(components)
			return components
		}
	}
	if name == "CreatedById" {
		return []string{name, "CreatedDate"}
	}
	if name == "LastModifiedById" {
		return []string{name, "LastModifiedDate"}
	}
	return []string{name}
}

func createDefaultsObjectInfos(org *storage.OrgState, objectName string, def storage.ObjectDefinition, source SourceMetadata, namespace string, info map[string]any) map[string]any {
	infos := map[string]any{objectName: info}
	names := createableFieldNames(def)
	if layout, ok := sourceCreateLayout(source, objectName); ok {
		names = nil
		for _, section := range layout.Sections {
			for _, col := range section.Columns {
				for _, item := range col.Items {
					if name, ok := storage.ResolveFieldName(def, namespace, item.Field); ok {
						names = append(names, name)
					}
				}
			}
		}
	}
	for _, name := range names {
		if name == "RecordTypeId" {
			continue
		}
		field := def.Fields[name]
		if field.Type != storage.FieldReference {
			continue
		}
		targets := append([]string(nil), field.ReferenceTo...)
		if len(targets) > 1 {
			targets = append(targets, "Name")
		}
		for _, target := range targets {
			if _, found := infos[target]; found {
				continue
			}
			if related, err := getObjectInfoWireData(org, target); err == nil {
				infos[target] = related
			}
		}
	}
	return infos
}
