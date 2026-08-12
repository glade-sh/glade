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
	localTabs := vm.schemaDescribeTabValues()
	tabSets := make([]Value, 0, len(defaultDescribeTabSetTemplates))
	for index, template := range defaultDescribeTabSetTemplates {
		tabs := vm.describeDefaultTabValues(template.Tabs)
		if index == len(defaultDescribeTabSetTemplates)-1 {
			tabs = append(tabs, localTabs...)
		}
		tabSets = append(tabSets, describeTabSetValue(template, tabs))
	}
	value := List(tabSets...)
	vm.describeTabsCache = &value
	return value
}

type describeTabSetTemplate struct {
	Name        string
	Label       string
	Description string
	Namespace   string
	Selected    bool
	Tabs        []string
}

var defaultDescribeTabSetTemplates = []describeTabSetTemplate{
	{Name: "Sales", Label: "Sales", Description: "The world's most popular sales force automation (SFA) solution", Namespace: "standard", Selected: true, Tabs: []string{"Home", "Chatter", "Campaigns", "Leads", "Accounts", "Contacts", "Opportunities", "Forecasts", "Contracts", "Orders", "Cases", "Solutions", "Products", "Reports", "Dashboards"}},
	{Name: "Service", Label: "Service", Description: "Manage customer service with accounts, contacts, cases, and more", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Accounts", "Contacts", "Cases", "Solutions", "Reports", "Dashboards"}},
	{Name: "MarketingCRMClassic", Label: "Marketing CRM Classic", Description: "Track sales and marketing efforts with CRM objects.", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Campaigns", "Leads", "Contacts", "Opportunities", "Reports", "Dashboards"}},
	{Name: "HighVolumeCustomerPortalUser", Label: "High Volume Customer Portal User", Tabs: []string{"Home"}},
	{Name: "AuthenticatedWebsiteUser", Label: "Authenticated Website User", Tabs: []string{"Home"}},
	{Name: "AppLauncher", Label: "App Launcher", Description: "App Launcher tabs", Namespace: "standard", Tabs: []string{"App Launcher"}},
	{Name: "Community", Label: "Community", Description: "Salesforce CRM Communities", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Contacts", "Accounts", "Ideas", "Reports", "Dashboards"}},
	{Name: "SiteCom", Label: "Site.com", Description: "Build pixel-perfect, data-rich websites using the drag-and-drop Site.com application, and manage content and published sites.", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Site.com"}},
	{Name: "SalesforceChatter", Label: "Salesforce Chatter", Description: "The Salesforce Chatter social network, including profiles and feeds", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Profile", "People", "Groups", "Files"}},
	{Name: "ProfileSelf", Label: "Profile (Self)", Description: "The tabs displayed when users view their own profile", Tabs: []string{"Profile Feed", "Profile Overview", "Recognition"}},
	{Name: "ProfileOthers", Label: "Profile (Others)", Description: "The tabs displayed when users view someone else's profile", Tabs: []string{"Profile Feed", "Profile Overview", "Recognition"}},
	{Name: "Content", Label: "Content", Description: "Salesforce CRM Content", Namespace: "standard", Tabs: []string{"Home", "Chatter", "Libraries", "Content", "Subscriptions"}},
	{Name: "AllTabs", Label: "All Tabs", Description: "All Tabs", Tabs: []string{"Home", "Contact Point Type Consent", "Data Use Purpose", "Data Use Legal Basis", "Authorization Form", "Authorization Form Consent", "Authorization Form Data Use", "Authorization Form Text", "Communication Subscriptions", "Engagement Channel Types", "Communication Subscription Channel Types", "Communication Subscription Consents", "Communication Subscription Timings", "Party Consent"}},
}

func (vm *VM) describeDefaultTabValues(names []string) []Value {
	values := make([]Value, 0, len(names))
	for _, name := range names {
		values = append(values, vm.describeTabValue(storage.TabMetadata{Name: name, Label: name, SObjectName: defaultDescribeTabSObjectName(name)}))
	}
	return values
}

func defaultDescribeTabSObjectName(name string) string {
	switch name {
	case "Campaigns":
		return "Campaign"
	case "Leads":
		return "Lead"
	case "Accounts":
		return "Account"
	case "Contacts":
		return "Contact"
	case "Opportunities":
		return "Opportunity"
	case "Forecasts":
		return "Forecasting3"
	case "Contracts":
		return "Contract"
	case "Orders":
		return "Order"
	case "Cases":
		return "Case"
	case "Solutions":
		return "Solution"
	case "Products":
		return "Product2"
	case "Reports":
		return "Report"
	case "Dashboards":
		return "Dashboard"
	case "App Launcher":
		return "AppLauncher"
	case "Ideas":
		return "Idea"
	case "Site.com":
		return "Sites"
	case "Profile", "People":
		return "User"
	case "Groups":
		return "CollaborationGroup"
	case "Files":
		return "File"
	case "Profile Feed":
		return "ProfilePlatformFeed"
	case "Profile Overview":
		return "ProfilePlatformOverview"
	case "Libraries":
		return "Workspace"
	case "Content":
		return "ContentSearch"
	case "Subscriptions":
		return "ContentSubscriptions"
	case "Contact Point Type Consent":
		return "ContactPointTypeConsent"
	case "Data Use Purpose":
		return "DataUsePurpose"
	case "Data Use Legal Basis":
		return "DataUseLegalBasis"
	case "Authorization Form":
		return "AuthorizationForm"
	case "Authorization Form Consent":
		return "AuthorizationFormConsent"
	case "Authorization Form Data Use":
		return "AuthorizationFormDataUse"
	case "Authorization Form Text":
		return "AuthorizationFormText"
	case "Communication Subscriptions":
		return "CommSubscription"
	case "Communication Subscription Channel Types":
		return "CommSubscriptionChannelType"
	case "Communication Subscription Consents":
		return "CommSubscriptionConsent"
	case "Communication Subscription Timings":
		return "CommSubscriptionTiming"
	case "Party Consent":
		return "PartyConsent"
	default:
		return name
	}
}

func describeTabSetValue(template describeTabSetTemplate, tabs []Value) Value {
	tabSet := Object("Schema.DescribeTabSetResult")
	tabSet.Fields["name"] = String(template.Name)
	tabSet.Fields["label"] = String(template.Label)
	tabSet.Fields["description"] = String(template.Description)
	tabSet.Fields["logoUrl"] = Null
	if template.Namespace == "" {
		tabSet.Fields["namespace"] = Null
	} else {
		tabSet.Fields["namespace"] = String(template.Namespace)
	}
	tabSet.Fields["tabSetId"] = String(template.Name)
	tabSet.Fields["tabs"] = List(tabs...)
	tabSet.Fields["selected"] = Bool(template.Selected)
	return tabSet
}
func (vm *VM) schemaDescribeTabValues() []Value {
	if vm.Org == nil {
		return nil
	}
	tabs := append([]storage.TabMetadata(nil), vm.Org.Metadata.Tabs...)
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
	value.Fields["colors"] = List(describeTabColorValue(tab))
	value.Fields["miniIconUrl"] = String(describeTabIconURL(tab))
	value.Fields["url"] = String(describeTabURL(tab))
	value.Fields["mobileUrl"] = value.Fields["url"]
	value.Fields["tabEnumOrId"] = String(tab.Name)
	return value
}

func describeTabURL(tab storage.TabMetadata) string {
	return "/lightning/o/" + tab.Name + "/list"
}

func describeTabColorValue(tab storage.TabMetadata) Value {
	color := Object("Schema.DescribeColorResult")
	color.Fields["color"] = String("#747474")
	color.Fields["context"] = String("primary")
	color.Fields["theme"] = String(tab.Motif)
	return color
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

func (vm *VM) hasDataCategoryMetadata() bool {
	return vm != nil && vm.Org != nil && len(vm.Org.Metadata.DataCategoryGroups) > 0
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
