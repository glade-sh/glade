package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) schemaGlobalDescribeAliasShouldReplace(alias, objectName string, existing Value) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(vm.Org.Namespace) == "" {
		return true
	}
	existingObject, ok := existing.Fields["object"]
	if !ok || existingObject.Kind != ValueString {
		return true
	}
	if strings.EqualFold(alias, objectName) {
		return true
	}
	namespace := vm.Org.Namespace
	candidateCurrent := !strings.EqualFold(storage.StripNamespaceToken(namespace, objectName), objectName)
	existingCurrent := !strings.EqualFold(storage.StripNamespaceToken(namespace, existingObject.Text), existingObject.Text)
	switch {
	case candidateCurrent && !existingCurrent:
		return true
	case !candidateCurrent && existingCurrent:
		return false
	default:
		return true
	}
}
func (vm *VM) schemaDescribeObjectName(value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && value.Type == "Schema.SObjectType" {
		objectValue, ok := value.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return "", fmt.Errorf("Schema.SObjectType token missing object")
		}
		return objectValue.Text, nil
	}
	return "", fmt.Errorf("Schema.describeSObjects expects object names or SObjectType tokens")
}
func (vm *VM) schemaDescribeTabs() Value {
	if vm.describeTabsCache != nil {
		return *vm.describeTabsCache
	}
	tabs := vm.schemaDescribeTabValues()
	if len(tabs) == 0 {
		value := List()
		vm.describeTabsCache = &value
		return value
	}
	tabSet := Object("Schema.DescribeTabSetResult")
	tabSet.Fields["name"] = String("AllTabs")
	tabSet.Fields["label"] = String("All Tabs")
	tabSet.Fields["tabs"] = List(tabs...)
	tabSet.Fields["selected"] = Bool(false)
	value := List(tabSet)
	vm.describeTabsCache = &value
	return value
}
func (vm *VM) schemaDescribeTabValues() []Value {
	if vm.Org == nil {
		return nil
	}
	tabs := append([]storage.TabMetadata(nil), vm.Org.Metadata.Tabs...)
	seen := make(map[string]struct{}, len(tabs))
	for _, tab := range tabs {
		if objectName := describeTabSObjectName(tab); objectName != "" {
			seen[strings.ToLower(objectName)] = struct{}{}
		}
	}
	objectNames := make([]string, 0, len(vm.Org.Objects))
	for name, state := range vm.Org.Objects {
		apiName := state.Definition.APIName
		if apiName == "" {
			apiName = name
		}
		if !isStandardDescribeTabObject(apiName) {
			continue
		}
		key := strings.ToLower(apiName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		objectNames = append(objectNames, apiName)
	}
	sort.Strings(objectNames)
	for _, name := range objectNames {
		tabs = append(tabs, storage.TabMetadata{
			Name:        name,
			Label:       name,
			SObjectName: name,
		})
	}
	sort.Slice(tabs, func(i, j int) bool { return tabs[i].Name < tabs[j].Name })
	values := make([]Value, 0, len(tabs))
	for _, tab := range tabs {
		if describeTabSObjectName(tab) == "" {
			continue
		}
		values = append(values, vm.describeTabValue(tab))
	}
	return values
}
func isStandardDescribeTabObject(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	if lowered == "" {
		return false
	}
	for _, suffix := range []string{"__c", "__e", "__mdt", "__b", "__x"} {
		if strings.HasSuffix(lowered, suffix) {
			return false
		}
	}
	_, ok := standardDescribeTabObjects[lowered]
	return ok
}
func (vm *VM) describeTabValue(tab storage.TabMetadata) Value {
	value := Object("Schema.DescribeTabResult")
	label := tab.Label
	if label == "" {
		label = tab.Name
	}
	value.Fields["name"] = String(tab.Name)
	value.Fields["label"] = String(label)
	sObjectName := describeTabSObjectName(tab)
	if sObjectName == "" {
		value.Fields["sObjectName"] = Null
	} else {
		sObjectName = vm.describeObjectName(sObjectName)
		value.Fields["sObjectName"] = String(sObjectName)
	}
	value.Fields["custom"] = Bool(tab.Custom)
	value.Fields["iconUrl"] = String(tab.Motif)
	value.Fields["icons"] = List(describeTabIconValue(tab))
	value.Fields["url"] = String("/lightning/o/" + tab.Name + "/list")
	return value
}
func describeTabSObjectName(tab storage.TabMetadata) string {
	sObjectName := strings.TrimSpace(tab.SObjectName)
	if sObjectName == "" && tab.Custom && hasSuffixFold(tab.Name, "__c") {
		sObjectName = tab.Name
	}
	return sObjectName
}
func describeTabIconValue(tab storage.TabMetadata) Value {
	icon := Object("Schema.DescribeIconResult")
	icon.Fields["contentType"] = String("image/svg+xml")
	icon.Fields["height"] = Int(0)
	icon.Fields["theme"] = String(tab.Motif)
	icon.Fields["url"] = String(describeTabIconURL(tab))
	icon.Fields["width"] = Int(0)
	return icon
}
func describeTabIconURL(tab storage.TabMetadata) string {
	name := strings.TrimSpace(tab.Name)
	if name == "" {
		name = "custom"
	}
	token := "custom"
	if tab.Custom {
		token = strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(name, "__c"), "__tab"))
		token = strings.ReplaceAll(token, "__", "_")
	}
	return "/img/icon/t4v35/custom/" + token + "_120.png.svg"
}
func (vm *VM) schemaDescribeDataCategoryGroups(sobjects Value) Value {
	out := typedList("List<Schema.DescribeDataCategoryGroupResult>")
	if vm.Org == nil {
		return out
	}
	for _, sobject := range sobjects.List {
		if sobject.Kind != ValueString {
			continue
		}
		for _, group := range vm.dataCategoryGroupsForSObject(sobject.Text) {
			out.List = append(out.List, describeDataCategoryGroupValue(group))
		}
	}
	return out
}
func (vm *VM) schemaDescribeDataCategoryGroupStructures(pairs Value, topCategoriesOnly bool) Value {
	out := typedList("List<Schema.DescribeDataCategoryGroupStructureResult>")
	if vm.Org == nil {
		return out
	}
	for _, pair := range pairs.List {
		if pair.Kind != ValueObject {
			continue
		}
		sobjectName := schemaPassiveStringField(pair, "sobject")
		groupName := schemaPassiveStringField(pair, "dataCategoryGroupName")
		for _, group := range vm.dataCategoryGroupsForSObject(sobjectName) {
			if !vmMetadataNameMatches(group.Name, groupName) {
				continue
			}
			out.List = append(out.List, describeDataCategoryGroupStructureValue(group, topCategoriesOnly))
		}
	}
	return out
}
func (vm *VM) dataCategoryGroupsForSObject(sobjectName string) []storage.DataCategoryGroup {
	if vm == nil || vm.Org == nil {
		return nil
	}
	groups := make([]storage.DataCategoryGroup, 0)
	for _, group := range vm.Org.Metadata.DataCategoryGroups {
		if !vmMetadataNameMatches(group.SObjectName, sobjectName) {
			continue
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}
func describeDataCategoryGroupValue(group storage.DataCategoryGroup) Value {
	value := Object("Schema.DescribeDataCategoryGroupResult")
	value.Fields["name"] = String(group.Name)
	value.Fields["label"] = String(metadataLabel(group.Label, group.Name))
	value.Fields["description"] = String(group.Description)
	value.Fields["sobject"] = String(group.SObjectName)
	value.Fields["categorycount"] = Int(int64(countDataCategories(group.Categories)))
	return value
}
func describeDataCategoryGroupStructureValue(group storage.DataCategoryGroup, topCategoriesOnly bool) Value {
	value := Object("Schema.DescribeDataCategoryGroupStructureResult")
	value.Fields["name"] = String(group.Name)
	value.Fields["label"] = String(metadataLabel(group.Label, group.Name))
	value.Fields["description"] = String(group.Description)
	value.Fields["sobject"] = String(group.SObjectName)
	categories := typedList("List<Schema.DataCategory>")
	for _, category := range group.Categories {
		categories.List = append(categories.List, describeDataCategoryValue(category, topCategoriesOnly))
	}
	value.Fields["topcategories"] = categories
	return value
}
func describeDataCategoryValue(category storage.DataCategory, topCategoriesOnly bool) Value {
	value := Object("Schema.DataCategory")
	value.Fields["name"] = String(category.Name)
	value.Fields["label"] = String(metadataLabel(category.Label, category.Name))
	children := typedList("List<Schema.DataCategory>")
	if !topCategoriesOnly {
		for _, child := range category.Children {
			children.List = append(children.List, describeDataCategoryValue(child, false))
		}
	}
	value.Fields["childcategories"] = children
	return value
}
func countDataCategories(categories []storage.DataCategory) int {
	count := 0
	for _, category := range categories {
		count++
		count += countDataCategories(category.Children)
	}
	return count
}
func schemaPassiveStringField(value Value, field string) string {
	_, fieldValue, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	if fieldValue.Kind == ValueString {
		return fieldValue.Text
	}
	return ""
}
func metadataLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}
func vmMetadataNameMatches(candidate, requested string) bool {
	return strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(requested))
}
