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
)

type CommunityContext struct {
	Site      string `json:"site,omitempty"`
	BasePath  string `json:"basePath,omitempty"`
	SiteID    string `json:"siteId,omitempty"`
	NetworkID string `json:"networkId,omitempty"`
	Guest     bool   `json:"guest,omitempty"`
	Language  string `json:"language,omitempty"`
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
