package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaAutomationDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA035", Message: message, File: typ.File, Range: semaRange(source, start, end)}
}

// C016/H001-H005: qualified source classes do not inherit restrictions from
// the platform namespace they shadow. Unresolved hosted names still receive
// their captured unavailability diagnostics below. C013-C015: schema entries
// such as the Approval SObject are not source-class shadows of a namespace.
func semaAutomationSourceType(model *semaTypeMemberView, typeName string) bool {
	members, _, resolved := semaLookupTypeMembers(model, typeName)
	return resolved && !members.platform && !members.sobject
}

// These symbols are not public Apex types in
// either captured API. Reflection remains separate from source visibility.
func semaAutomationTypeMessage(typeName string, model *semaTypeMemberView) string {
	if semaAutomationSourceType(model, typeName) {
		return ""
	}
	if message := semaConnectAPIFeatureGateMessage(typeName, model); message != "" {
		return message
	}
	for _, name := range []string{"lxscheduler.GetAppointmentCandidatesInput", "lxscheduler.GetAppointmentSlotsInput", "lxscheduler.SkillRequirement", "lxscheduler.WorkType", "lxscheduler.SchedulerResources", "KbManagement.PublishingService"} {
		if strings.EqualFold(typeName, name) {
			return "Type is not visible: " + name
		}
	}
	if strings.EqualFold(typeName, "FSL.ScheduleResult") {
		return "Invalid type: FSL.ScheduleResult"
	}
	return ""
}

func semaAutomationCallTypeMessage(typeName, method string, model *semaTypeMemberView) string {
	// Publishing is available only in a Knowledge-enabled org. Static receiver
	// visibility remains separate from declaring a variable of this type, and
	// ordinary member resolution still checks the method's argument types.
	if model != nil && model.state != nil && model.state.base != nil && model.state.base.knowledgeEnabled &&
		strings.EqualFold(typeName, "KbManagement.PublishingService") &&
		(strings.EqualFold(method, "publishArticle") || strings.EqualFold(method, "archiveOnlineArticle") || strings.EqualFold(method, "assignDraftArticleTask") || strings.EqualFold(method, "editOnlineArticle")) {
		return ""
	}
	return semaAutomationTypeMessage(typeName, model)
}

// The invisible input type is the receiver of an inner
// builder() call. Walk receivers for automation visibility checks.
func semaAutomationChainedTypeMessage(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) string {
	for current := &expr; current != nil; current = current.Left {
		if current.Kind != ir.ExprCall {
			continue
		}
		receiver, method, qualified := splitSemaMethodPath(current.Callee)
		if !qualified && current.Left != nil && current.Left.Kind == ir.ExprVariable {
			receiver, method, qualified = current.Left.Name, current.Callee, true
		}
		if !qualified {
			continue
		}
		root, _, _ := strings.Cut(receiver, ".")
		if _, scoped := scope.lookup(root); scoped {
			continue
		}
		if message := semaAutomationCallTypeMessage(receiver, method, model); message != "" {
			return message
		}
	}
	return ""
}

func semaAutomationLocalDiagnostics(diagnostics []diagnostic.Diagnostic, typ typesys.TypeSymbol, scan *semaBodyExpressionScan, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	body := scan.body
	for _, match := range scan.localDeclMatches {
		if semaLocalDeclMatchInIgnoredText(scan.ignored, match) {
			continue
		}
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		message := semaAutomationTypeMessage(typeName, model)
		if message == "" {
			continue
		}
		start, end := bodyOffset+match[2], bodyOffset+match[3]
		replaced := false
		for i := range diagnostics {
			if diagnostics[i].Range != nil && diagnostics[i].Range.Start.Offset == start && diagnostics[i].Range.End.Offset == end {
				diagnostics[i] = semaAutomationDiagnostic(typ, message, start, end, source)
				replaced = true
			}
		}
		if !replaced {
			diagnostics = append(diagnostics, semaAutomationDiagnostic(typ, message, start, end, source))
		}
	}
	return diagnostics
}

func semaAutomationConstructorMessage(typeName string, model *semaTypeMemberView) string {
	if semaAutomationSourceType(model, typeName) {
		return ""
	}
	for _, name := range []string{"Approval.ProcessRequest", "Approval.ProcessResult", "Approval.LockResult", "Approval.UnlockResult", "QuickAction.QuickActionResult"} {
		if strings.EqualFold(typeName, name) {
			return "Type cannot be constructed: " + name
		}
	}
	if strings.EqualFold(typeName, "Flow.Interview") {
		return "Abstract classes cannot be constructed: Flow.Interview"
	}
	if strings.EqualFold(typeName, "Invocable.Action") {
		return "Constructor not defined: [Invocable.Action].<Constructor>()"
	}
	return ""
}

func (a *Analyzer) semaAutomationCallMessage(receiverType, method string, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	if semaAutomationSourceType(model, receiverType) {
		return ""
	}
	if message := semaAutomationCallTypeMessage(receiverType, method, model); message != "" {
		return message
	}
	if strings.EqualFold(receiverType, "FSL.ScheduleService") {
		return "Variable does not exist: FSL.ScheduleService"
	}
	var params [][]string
	canonicalType := receiverType
	switch normalizeName(receiverType) {
	case "kbmanagement.publishingservice":
		if strings.EqualFold(method, "archiveOnlineArticle") {
			canonicalType = "KbManagement.PublishingService"
			params = [][]string{{"String", "Datetime"}}
		}
	case "approval":
		canonicalType = "Approval"
		switch normalizeName(method) {
		case "lock", "unlock", "islocked":
			if sig, ok := semaPlatformMethodSignature(receiverType, method); ok {
				params = sig.params
			}
		}
	case "approval.processsubmitrequest", "approval.processworkitemrequest", "approval.processrequest":
		if strings.EqualFold(method, "setNextApproverIds") {
			params = [][]string{{"List<Id>"}}
		}
		if strings.EqualFold(receiverType, "Approval.ProcessSubmitRequest") && strings.EqualFold(method, "setSkipEntryCriteria") {
			params = [][]string{{"Boolean"}}
		}
		if strings.EqualFold(receiverType, "Approval.ProcessWorkitemRequest") && strings.EqualFold(method, "setAction") {
			params = [][]string{{"String"}}
		}
	case "flow.interview":
		canonicalType = "Flow.Interview"
		switch normalizeName(method) {
		case "createinterview":
			params = [][]string{{"String", "Map<String,Object>"}, {"String", "String", "Map<String,Object>"}}
		case "start":
			params = [][]string{{}}
		case "setvariablevalue":
			return "Method is not visible: void Flow.Interview.setVariableValue(String, Object)"
		}
	case "invocable.action":
		canonicalType = "Invocable.Action"
		switch normalizeName(method) {
		case "setinvocationparameter":
			params = [][]string{{"String", "Object"}}
		case "setinvocations":
			params = [][]string{{"List<Map<String,Object>>"}}
		}
	case "quickaction.quickactionrequest":
		canonicalType = "QuickAction.QuickActionRequest"
		switch normalizeName(method) {
		case "setrecord":
			params = [][]string{{"SObject"}}
		case "setcontextid":
			params = [][]string{{"Id"}}
		case "setquickactionname":
			params = [][]string{{"String"}}
		}
	}
	if params == nil {
		return ""
	}
	argTypes := irCallArgTypes(a, args, scope, model, currentType)
	if semaArgsMatchAny(params, argTypes, model) {
		return ""
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ", "), canonicalType)
}

// capi2_feature_gates/org.tsv and capi3_feature_gates/org.tsv: each service
// compiles only in a scratch shape that enables every listed feature or
// setting. The name is the platform's own casing. A DTO type is never gated:
// only the static service classes are hidden.
type semaConnectAPIFeatureGate struct {
	name     string
	features []string
}

var semaConnectAPIFeatureGates = buildSemaConnectAPIFeatureGates(map[string][]string{
	"ConnectApi.OmniDesignerConnect": {"OmniStudioDesigner"},
	"ConnectApi.EinsteinLLM":         {"Einstein1AIPlatform"},
	// The Knowledge setting, not the license feature, enables it;
	// project.scratchDefinitionFeatures emits "Knowledge" only with
	// knowledgeSettings.enableKnowledge, as for the Knowledge objects.
	"ConnectApi.Knowledge": {"Knowledge"},
	// CRM Analytics and Data Cloud services appear with DevelopmentWave;
	// AnalyticsAdminPerms does not.
	"ConnectApi.Wave":               {"DevelopmentWave"},
	"ConnectApi.CdpQuery":           {"DevelopmentWave"},
	"ConnectApi.CdpCatalog":         {"DevelopmentWave"},
	"ConnectApi.SmartDataDiscovery": {"DevelopmentWave"},
	// The Experience Cloud networks setting (communitiesSettings.
	// enableNetworksEnabled, the NetworksEnabled token); the Communities
	// feature alone does not enable them (capi3_feature_gates, shape
	// "features=Communities").
	"ConnectApi.NavigationMenu":         {"NetworksEnabled"},
	"ConnectApi.Personalization":        {"NetworksEnabled"},
	"ConnectApi.ExternalManagedAccount": {"NetworksEnabled"},
	"ConnectApi.MarketingIntegration":   {"NetworksEnabled"},
	"ConnectApi.Sites":                  {"NetworksEnabled"},
	// Order Management: the feature opens the first group, its setting the
	// second, enhanced orders the third (OrderSummaryCreation needs only the
	// enhanced setting), and both settings the last.
	"ConnectApi.CommerceCart":         {"OrderManagement"},
	"ConnectApi.CommerceInventory":    {"OrderManagement"},
	"ConnectApi.CommercePromotions":   {"OrderManagement"},
	"ConnectApi.RegisterGuestBuyer":   {"OrderManagement"},
	"ConnectApi.Repricing":            {"OrderManagement"},
	"ConnectApi.ReturnOrder":          {"OrderManagement"},
	"ConnectApi.OMSAnalytics":         {"OrderManagement", "OrderManagementEnabled"},
	"ConnectApi.PickTicket":           {"OrderManagement", "OrderManagementEnabled"},
	"ConnectApi.Routing":              {"OrderManagement", "OrderManagementEnabled"},
	"ConnectApi.EnsurePaymentCredit":  {"OrderManagement", "EnhancedCommerceOrders"},
	"ConnectApi.Exchanges":            {"OrderManagement", "EnhancedCommerceOrders"},
	"ConnectApi.OrderSummaryCreation": {"EnhancedCommerceOrders"},
	"ConnectApi.OrderSummary":         {"OrderManagement", "OrderManagementEnabled", "EnhancedCommerceOrders"},
	"ConnectApi.FulfillmentOrder":     {"OrderManagement", "OrderManagementEnabled", "EnhancedCommerceOrders"},
	"ConnectApi.OrderPaymentSummary":  {"OrderManagement", "OrderManagementEnabled", "EnhancedCommerceOrders"},
})

func buildSemaConnectAPIFeatureGates(table map[string][]string) map[string]semaConnectAPIFeatureGate {
	out := make(map[string]semaConnectAPIFeatureGate, len(table))
	for name, features := range table {
		out[strings.ToLower(name)] = semaConnectAPIFeatureGate{name: name, features: features}
	}
	return out
}

// semaConnectAPIFeatureGateMessage hides a gated service only when the project
// declares an org shape that lacks the feature. Without a declared shape the
// service stays visible, because the real org is unknown.
func semaConnectAPIFeatureGateMessage(typeName string, model *semaTypeMemberView) string {
	gate, gated := semaConnectAPIFeatureGates[strings.ToLower(typeName)]
	if !gated || model == nil || model.state == nil || model.state.base == nil {
		return ""
	}
	features := model.state.base.orgShapeFeatures
	if len(features) == 0 {
		return ""
	}
	// The token is appended to positive evidence (Person Accounts, dependency
	// scratch definitions) when the project declares no shape, so it can sit
	// beside real features.
	for _, feature := range features {
		if feature == project.UnspecifiedStandardFieldShape {
			return ""
		}
	}
	for _, required := range gate.features {
		found := false
		for _, feature := range features {
			if strings.EqualFold(feature, required) {
				found = true
				break
			}
		}
		if !found {
			return "Type is not visible: " + gate.name
		}
	}
	return ""
}
