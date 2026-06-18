package lwcshell

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type FlexiPage struct {
	Name          string       `json:"name"`
	Label         string       `json:"label,omitempty"`
	Type          string       `json:"type,omitempty"`
	ObjectAPIName string       `json:"objectApiName,omitempty"`
	Template      string       `json:"template,omitempty"`
	File          string       `json:"file,omitempty"`
	Regions       []PageRegion `json:"regions,omitempty"`
}

type RenderTargetKind string

const (
	RenderTargetComponent      RenderTargetKind = "component"
	RenderTargetRecordPage     RenderTargetKind = "recordPage"
	RenderTargetAppPage        RenderTargetKind = "appPage"
	RenderTargetHomePage       RenderTargetKind = "homePage"
	RenderTargetTab            RenderTargetKind = "tab"
	RenderTargetURLAddressable RenderTargetKind = "urlAddressable"
	RenderTargetQuickAction    RenderTargetKind = "quickAction"
	RenderTargetCommunityPage  RenderTargetKind = "communityPage"
	RenderTargetUtilityBar     RenderTargetKind = "utilityBar"
	RenderTargetFlowScreen     RenderTargetKind = "flowScreen"
	RenderTargetFlowAction     RenderTargetKind = "flowAction"
)

type CommunityContext struct {
	Site           string                          `json:"site,omitempty"`
	BasePath       string                          `json:"basePath,omitempty"`
	SiteID         string                          `json:"siteId,omitempty"`
	NetworkID      string                          `json:"networkId,omitempty"`
	Guest          bool                            `json:"guest,omitempty"`
	Language       string                          `json:"language,omitempty"`
	RouteParams    map[string]string               `json:"routeParams,omitempty"`
	Menus          map[string][]CommunityMenuItem  `json:"menus,omitempty"`
	ManagedContent map[string]CommunityContentItem `json:"managedContent,omitempty"`
}

type CommunityMenuItem struct {
	Label      string `json:"label,omitempty"`
	Target     string `json:"target,omitempty"`
	Type       string `json:"type,omitempty"`
	ContentKey string `json:"contentKey,omitempty"`
}

type CommunityContentItem struct {
	ContentKey string `json:"contentKey,omitempty"`
	Title      string `json:"title,omitempty"`
	Body       string `json:"body,omitempty"`
	ImageURL   string `json:"imageUrl,omitempty"`
}

type WorkspaceContext struct {
	Console      bool           `json:"console,omitempty"`
	FocusedTabID string         `json:"focusedTabId,omitempty"`
	Tabs         []WorkspaceTab `json:"tabs,omitempty"`
	Utilities    []UtilityItem  `json:"utilities,omitempty"`
}

type WorkspaceTab struct {
	TabID        string `json:"tabId,omitempty"`
	Label        string `json:"label,omitempty"`
	URL          string `json:"url,omitempty"`
	WorkspaceTab bool   `json:"workspaceTab,omitempty"`
}

type UtilityItem struct {
	ID            string `json:"id,omitempty"`
	Label         string `json:"label,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	URL           string `json:"url,omitempty"`
}

type FlowContext struct {
	APIName          string         `json:"apiName,omitempty"`
	InputVariables   map[string]any `json:"inputVariables,omitempty"`
	AvailableActions []string       `json:"availableActions,omitempty"`
}

type PageContext struct {
	Kind          RenderTargetKind  `json:"kind"`
	ComponentName string            `json:"componentName,omitempty"`
	PageName      string            `json:"pageName,omitempty"`
	RecordID      string            `json:"recordId,omitempty"`
	ObjectAPIName string            `json:"objectApiName,omitempty"`
	AppName       string            `json:"appName,omitempty"`
	TabName       string            `json:"tabName,omitempty"`
	ActionName    string            `json:"actionName,omitempty"`
	ActionType    string            `json:"actionType,omitempty"`
	FormFactor    string            `json:"formFactor,omitempty"`
	State         map[string]string `json:"state,omitempty"`
	Community     CommunityContext  `json:"community,omitempty"`
	Workspace     WorkspaceContext  `json:"workspace,omitempty"`
	Flow          FlowContext       `json:"flow,omitempty"`
	PageReference map[string]any    `json:"pageReference,omitempty"`
}

type ShellPage struct {
	Context     PageContext    `json:"context"`
	Page        FlexiPage      `json:"page,omitempty"`
	Tab         CustomTab      `json:"tab,omitempty"`
	ThemeLayout *PageComponent `json:"themeLayout,omitempty"`
	Regions     []PageRegion   `json:"regions,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

type PageRegion struct {
	Name       string          `json:"name"`
	Type       string          `json:"type,omitempty"`
	Components []PageComponent `json:"components,omitempty"`
}

type PageComponent struct {
	ComponentName     string            `json:"componentName"`
	Identifier        string            `json:"identifier,omitempty"`
	Properties        map[string]string `json:"properties,omitempty"`
	Kind              string            `json:"kind,omitempty"`
	UnsupportedReason string            `json:"unsupportedReason,omitempty"`
}

type TabType string

const (
	TabTypeUnknown     TabType = ""
	TabTypeLWC         TabType = "lwc"
	TabTypeFlexiPage   TabType = "flexipage"
	TabTypeVisualforce TabType = "visualforce"
	TabTypeWeb         TabType = "web"
	TabTypeObject      TabType = "object"
)

type CustomTab struct {
	Name   string  `json:"name"`
	Label  string  `json:"label,omitempty"`
	Type   TabType `json:"type,omitempty"`
	Target string  `json:"target,omitempty"`
	File   string  `json:"file,omitempty"`
}

type QuickAction struct {
	Name          string `json:"name"`
	Label         string `json:"label,omitempty"`
	Type          string `json:"type,omitempty"`
	TargetObject  string `json:"targetObject,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	ActionType    string `json:"actionType,omitempty"`
	File          string `json:"file,omitempty"`
}

func (t CustomTab) UnsupportedDiagnostic() Diagnostic {
	switch t.Type {
	case TabTypeVisualforce:
		return Diagnostic{}
	case TabTypeWeb:
		return Diagnostic{Code: "GLADELWC007", Message: "web custom tabs are not supported by the LWC shell"}
	case TabTypeObject:
		return Diagnostic{Code: "GLADELWC007", Message: "object custom tabs are not supported by the LWC shell"}
	case TabTypeUnknown:
		return Diagnostic{Code: "GLADELWC007", Message: "custom tab target is unsupported"}
	default:
		return Diagnostic{}
	}
}
