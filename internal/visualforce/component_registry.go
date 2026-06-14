package visualforce

import (
	"strings"
	"sync"
)

type ComponentStatus string

const (
	ComponentSupported   ComponentStatus = "supported"
	ComponentPartial     ComponentStatus = "partial"
	ComponentUnsupported ComponentStatus = "unsupported"
)

type ComponentSpec struct {
	Name       string
	Namespace  string
	Status     ComponentStatus
	Reason     string
	Attributes []string
	DocSource  string
	Render     func(*MarkupNode, *RenderContext) (string, error)
}

var (
	standardComponentSpecsOnce sync.Once
	standardComponentSpecs     map[string]ComponentSpec
)

func StandardComponentSpecs() map[string]ComponentSpec {
	standardComponentSpecsOnce.Do(func() {
		standardComponentSpecs = buildStandardComponentSpecs()
	})
	return cloneComponentSpecs(standardComponentSpecs)
}

func buildStandardComponentSpecs() map[string]ComponentSpec {
	catalog := StandardComponentCatalog()
	specs := make(map[string]ComponentSpec, len(catalog))
	catalogByName := make(map[string]ComponentCatalogEntry, len(catalog))
	for _, entry := range catalog {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		if !isComponentCatalogEntry(entry) {
			continue
		}
		key := catalogNameKey(name)
		catalogByName[key] = entry
		specs[key] = ComponentSpec{
			Name:       name,
			Namespace:  componentNamespace(name),
			Status:     ComponentUnsupported,
			Reason:     unsupportedComponentReason(entry),
			Attributes: catalogAttributeNames(entry.Attributes),
			DocSource:  entry.SourceFile,
		}
	}
	for name, spec := range currentStandardComponentSpecs() {
		spec.Name = name
		key := catalogNameKey(name)
		if spec.Namespace == "" {
			spec.Namespace = componentNamespace(name)
		}
		if entry, ok := catalogByName[key]; ok {
			if len(spec.Attributes) == 0 {
				spec.Attributes = catalogAttributeNames(entry.Attributes)
			}
			if spec.DocSource == "" {
				spec.DocSource = entry.SourceFile
			}
		}
		if spec.Status == "" {
			spec.Status = ComponentPartial
		}
		if spec.Status == ComponentPartial && strings.TrimSpace(spec.Reason) == "" {
			spec.Reason = partialComponentReason(key)
		}
		if spec.Status != ComponentSupported && strings.TrimSpace(spec.Reason) == "" {
			spec.Reason = "current local renderer covers a subset of documented behavior"
		}
		specs[key] = spec
	}
	return specs
}

func StandardComponentSpec(namespace, component string) (ComponentSpec, bool) {
	key := componentRegistryKey(namespace, component)
	if key == "" {
		return ComponentSpec{}, false
	}
	standardComponentSpecsOnce.Do(func() {
		standardComponentSpecs = buildStandardComponentSpecs()
	})
	spec, ok := standardComponentSpecs[key]
	return spec, ok
}

func cloneComponentSpecs(specs map[string]ComponentSpec) map[string]ComponentSpec {
	out := make(map[string]ComponentSpec, len(specs))
	for key, spec := range specs {
		spec.Attributes = append([]string(nil), spec.Attributes...)
		out[key] = spec
	}
	return out
}

func currentStandardComponentSpecs() map[string]ComponentSpec {
	return map[string]ComponentSpec{
		"apex:page":         {Status: ComponentPartial, Render: renderApexPage},
		"apex:outputText":   {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexOutput(n, c, false) }},
		"apex:outputField":  {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexOutput(n, c, true) }},
		"apex:outputFormat": {Status: ComponentPartial, Render: renderApexOutputFormat},
		"apex:outputPanel": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "outputPanel", c)
		}},
		"apex:outputLabel": {Status: ComponentPartial, Render: renderApexOutputLabel},
		"apex:pageBlock": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "bPageBlock", c)
		}},
		"apex:pageBlockSection": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "pbSubsection", c)
		}},
		"apex:pageBlockSectionItem": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "pbSectionItem", c)
		}},
		"apex:pageBlockButtons": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "pbButtonbar", c)
		}},
		"apex:pageBlockTable": {Status: ComponentPartial, Render: renderApexPageBlockTable},
		"apex:pageMessages":   {Status: ComponentPartial, Render: renderApexPageMessages},
		"apex:pageMessage":    {Status: ComponentPartial, Render: renderApexPageMessage},
		"apex:message":        {Status: ComponentPartial, Render: renderApexMessage},
		"apex:messages":       {Status: ComponentPartial, Render: renderApexPageMessages},
		"apex:outputLink":     {Status: ComponentPartial, Render: renderApexLink},
		"apex:form":           {Status: ComponentPartial, Render: renderApexForm},
		"apex:inputText":      {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexInputText(n, c, "text") }},
		"apex:inputTextarea":  {Status: ComponentPartial, Render: renderApexInputTextarea},
		"apex:inputSecret":    {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexInputText(n, c, "password") }},
		"apex:inputHidden":    {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexInputText(n, c, "hidden") }},
		"apex:inputCheckbox":  {Status: ComponentPartial, Render: renderApexInputCheckbox},
		"apex:inputField":     {Status: ComponentPartial, Render: renderApexInputField},
		"apex:commandButton":  {Status: ComponentPartial, Render: renderApexCommandButton},
		"apex:commandLink":    {Status: ComponentPartial, Render: renderApexCommandLink},
		"apex:selectList":     {Status: ComponentPartial, Render: renderApexSelectList},
		"apex:selectCheckboxes": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexSelectInputs(n, c, "checkbox", "selectCheckboxes")
		}},
		"apex:selectRadio": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexSelectInputs(n, c, "radio", "selectRadio")
		}},
		"apex:selectOption":  {Status: ComponentPartial, Render: renderChildren},
		"apex:selectOptions": {Status: ComponentPartial, Render: renderChildren},
		"apex:repeat":        {Status: ComponentPartial, Render: renderApexRepeat},
		"apex:dataTable":     {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexDataTable(n, c, false) }},
		"apex:dataList":      {Status: ComponentPartial, Render: renderApexDataList},
		"apex:panelGrid":     {Status: ComponentPartial, Render: renderApexPanelGrid},
		"apex:column":        {Status: ComponentPartial, Render: renderChildren},
		"apex:detail":        {Status: ComponentPartial, Render: renderApexDetail},
		"apex:relatedList": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "relatedlist", c)
		}},
		"apex:enhancedList": {Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) {
			return renderApexContainer(n, "div", "enhancedlist", c)
		}},
		"apex:actionFunction":   {Status: ComponentPartial, Render: renderApexActionFunction},
		"apex:actionSupport":    {Status: ComponentPartial, Render: renderApexActionSupport},
		"apex:actionRegion":     {Status: ComponentPartial, Render: renderApexActionRegion},
		"apex:actionStatus":     {Status: ComponentPartial, Render: renderApexActionStatus},
		"apex:actionPoller":     {Status: ComponentPartial, Render: renderApexActionPoller},
		"apex:variable":         {Status: ComponentPartial, Render: renderApexVariable},
		"apex:attribute":        {Status: ComponentPartial, Render: renderNoopComponent},
		"apex:stylesheet":       {Status: ComponentPartial, Render: renderApexStylesheet},
		"apex:includeScript":    {Status: ComponentPartial, Render: renderApexIncludeScript},
		"apex:includeLightning": {Status: ComponentPartial, Render: renderApexIncludeLightning},
		"apex:slds":             {Status: ComponentPartial, Render: renderApexSLDS},
		"apex:image":            {Status: ComponentPartial, Render: renderApexImage},
		"apex:iframe":           {Status: ComponentPartial, Render: renderApexIframe},
		"apex:inputFile": {
			Status: ComponentPartial,
			Render: renderApexInputFile,
			Reason: "partial: renders an HTML file input with Visualforce field name/id for local form posts; true Blob controller field assignment, multipart form enctype, upload validation, and exact Salesforce lifecycle remain incomplete",
		},
		"apex:remoteObjects": {Status: ComponentPartial, Render: renderApexRemoteObjects,
			Reason: "partial: emits a local Remote Objects client model scaffold and preserves visible children, but hosted Salesforce CRUD transport, describe metadata, permissions, and exact $VFSOBJ behavior are not complete"},
		"apex:remoteObjectModel": {Status: ComponentUnsupported,
			Reason: "apex:remoteObjectModel is only valid inside apex:remoteObjects and requires local Remote Objects CRUD dispatch"},
		"apex:remoteObjectField": {Status: ComponentUnsupported,
			Reason: "apex:remoteObjectField is only valid inside apex:remoteObjectModel and requires local Remote Objects CRUD dispatch"},
		"flow:interview":        {Namespace: "flow", Status: ComponentUnsupported, Reason: FlowInterviewUnsupportedDiagnostic()},
		"apex:composition":      {Status: ComponentPartial, Render: renderApexComposition},
		"apex:define":           {Status: ComponentPartial, Render: renderApexDefine},
		"apex:insert":           {Status: ComponentPartial, Render: renderApexInsert},
		"apex:include":          {Status: ComponentPartial, Render: renderApexInclude},
		"apex:component":        {Status: ComponentPartial, Render: renderChildren},
		"apex:componentBody":    {Status: ComponentPartial, Render: renderApexComponentBody},
		"apex:dynamicComponent": {Status: ComponentPartial, Render: renderApexDynamicComponent},
		"apex:param":            {Status: ComponentPartial, Render: renderNoopComponent},
		"apex:facet":            {Status: ComponentPartial, Render: renderChildren},
	}
}

func renderNoopComponent(_ *MarkupNode, _ *RenderContext) (string, error) {
	return "", nil
}

func componentNamespace(name string) string {
	if idx := strings.Index(name, ":"); idx > 0 {
		return name[:idx]
	}
	return ""
}

func isComponentCatalogEntry(entry ComponentCatalogEntry) bool {
	name := strings.TrimSpace(entry.Name)
	return strings.Contains(name, ":") && !strings.Contains(entry.SourceFile, "additional_")
}

func catalogNameKey(name string) string {
	namespace, component, ok := strings.Cut(strings.TrimSpace(name), ":")
	if !ok {
		return strings.ToLower(strings.TrimSpace(name))
	}
	return componentRegistryKey(namespace, component)
}

func componentRegistryKey(namespace, component string) string {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	component = strings.ToLower(strings.TrimSpace(component))
	if namespace == "" || component == "" {
		return ""
	}
	return namespace + ":" + component
}

func catalogAttributeNames(attrs []ComponentAttribute) []string {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		name := strings.TrimSpace(attr.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func partialComponentReason(key string) string {
	switch key {
	case "apex:page":
		return "partial: missing standard controller and record set lifecycle edge cases, exact Salesforce header/sidebar chrome, cache/CSP headers, and non-PDF renderAs parity"
	case "apex:form":
		return "partial: missing full Visualforce validation lifecycle, prependId edge cases, forceSSL, accept/target semantics, and exact form event behavior"
	case "apex:outputtext", "apex:outputfield", "apex:outputformat", "apex:outputlabel", "apex:outputpanel", "apex:outputlink":
		return "partial: missing complete documented HTML attribute/event passthrough, Salesforce label/help chrome, and full field formatting parity"
	case "apex:pagemessages", "apex:pagemessage", "apex:message", "apex:messages":
		return "partial: missing exact Salesforce message chrome, escape/layout options, severity styling, and field association edge cases"
	case "apex:pageblock", "apex:pageblockbuttons", "apex:pageblocksection", "apex:pageblocksectionitem", "apex:pageblocktable":
		return "partial: missing exact Salesforce pageBlock chrome, collapsible/help behavior, header/footer attributes, and table styling parity"
	case "apex:datatable", "apex:datalist", "apex:repeat", "apex:column":
		return "partial: missing full row/column styling, header/footer facets, pagination with first/rows edge cases, and table event passthrough"
	case "apex:inputtext", "apex:inputtextarea", "apex:inputsecret", "apex:inputhidden", "apex:inputcheckbox", "apex:inputfield":
		return "partial: missing full Visualforce conversion and validation, required-field messages, disabled/readonly edge cases, and complete event/style passthrough"
	case "apex:selectlist", "apex:selectcheckboxes", "apex:selectradio", "apex:selectoption", "apex:selectoptions":
		return "partial: missing full SelectOption object handling, validation, disabled/readonly edge cases, layout variants, and complete event/style passthrough"
	case "apex:commandbutton", "apex:commandlink", "apex:actionfunction", "apex:actionsupport", "apex:actionregion", "apex:actionstatus", "apex:actionpoller":
		return "partial: missing full AJAX/action lifecycle parity, status timing, timeout/focus handling, parameter ordering, and partial-processing edge cases"
	case "apex:detail", "apex:relatedlist", "apex:enhancedlist":
		return "partial: missing Salesforce standard-controller layout metadata, related list data paging/actions, inline edit, and list-view security parity"
	case "apex:stylesheet", "apex:includescript", "apex:includelightning", "apex:slds", "apex:image", "apex:iframe":
		return "partial: missing Salesforce static resource URL resolution, CSP/resource versioning, hosted Lightning bootstrap details, and complete HTML attribute passthrough"
	case "apex:composition", "apex:define", "apex:insert", "apex:include", "apex:component", "apex:componentbody", "apex:dynamiccomponent":
		return "partial: missing full custom component controller lifecycle, typed attribute conversion, access/allowDML enforcement, and template error parity"
	case "apex:param", "apex:facet", "apex:attribute", "apex:variable":
		return "partial: missing complete assignment timing, typed attribute validation, required attribute enforcement, and facet validation parity"
	default:
		return "partial: local renderer has a catalog entry but needs explicit missing behavior before full support can be claimed"
	}
}

func unsupportedComponentReason(entry ComponentCatalogEntry) string {
	switch catalogNameKey(entry.Name) {
	case "analytics:reportchart":
		return "requires live Analytics chart service"
	case "apex:areaseries", "apex:axis", "apex:barseries", "apex:chart", "apex:chartlabel", "apex:charttips", "apex:gaugeseries", "apex:legend", "apex:lineseries", "apex:pieseries", "apex:radarseries", "apex:scatterseries":
		return "requires local Visualforce charting runtime"
	case "apex:canvasapp":
		return "requires Canvas signed request and hosted app frame"
	case "apex:emailpublisher", "apex:logcallpublisher":
		return "requires Salesforce publisher action runtime and client submit service"
	case "apex:flash":
		return "requires deprecated Flash plugin runtime; local renderer does not emulate it"
	case "apex:inlineeditsupport":
		return "requires Visualforce inline edit client runtime and standard controller save lifecycle"
	case "apex:input":
		return "requires local generic typed input binding and validation renderer"
	case "apex:inputfile":
		return "requires local inputFile renderer and multipart upload lifecycle before renderer support can be claimed"
	case "apex:listviews":
		return "requires Salesforce list view metadata and data runtime"
	case "apex:map", "apex:mapinfowindow", "apex:mapmarker":
		return "requires local map widget runtime and geocoding data contract"
	case "apex:milestonetracker":
		return "requires Entitlement milestone service runtime"
	case "apex:panelbar", "apex:panelbaritem":
		return "requires local Visualforce panel bar widget runtime"
	case "apex:panelgrid", "apex:panelgroup":
		return "requires local Visualforce panel layout renderer with documented table/span behavior"
	case "apex:remoteobjects":
		return "apex:remoteObjects requires local Remote Objects CRUD dispatch before renderer support can be claimed"
	case "apex:remoteobjectmodel":
		return "apex:remoteObjectModel is only valid inside apex:remoteObjects and requires local Remote Objects CRUD dispatch"
	case "apex:remoteobjectfield":
		return "apex:remoteObjectField is only valid inside apex:remoteObjectModel and requires local Remote Objects CRUD dispatch"
	case "apex:scontrol":
		return "requires legacy s-control hosting runtime"
	case "apex:sectionheader":
		return "requires Salesforce page header chrome renderer"
	case "apex:tab", "apex:tabpanel":
		return "requires local Visualforce tab widget runtime"
	case "apex:toolbar", "apex:toolbargroup":
		return "requires local Visualforce toolbar widget renderer"
	case "apex:vote":
		return "requires Salesforce vote service runtime"
	case "flow:interview":
		return FlowInterviewUnsupportedDiagnostic()
	}
	switch strings.ToLower(componentNamespace(entry.Name)) {
	case "analytics":
		return "requires live Analytics chart service"
	case "chatter":
		return "requires Salesforce Chatter feed service runtime"
	case "chatteranswers":
		return "requires Chatter Answers community runtime"
	case "ideas":
		return "requires Salesforce Ideas community runtime"
	case "knowledge":
		return "requires Salesforce Knowledge service runtime"
	case "liveagent":
		return "requires Salesforce Live Agent chat service runtime"
	case "messaging":
		return "requires Visualforce email template render pipeline"
	case "site":
		return "requires Salesforce Sites runtime"
	case "social":
		return "requires Salesforce social profile service runtime"
	case "support":
		return "requires Service Cloud support runtime"
	case "topics":
		return "requires Salesforce Topics service runtime"
	case "wave":
		return "requires CRM Analytics runtime and dashboard service"
	default:
		return "requires explicit local Visualforce renderer classification before support can be claimed"
	}
}
