package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
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

// The invisible input type is the receiver of an inner
// builder() call. Walk receivers for automation visibility checks.
func semaAutomationChainedTypeMessage(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) string {
	for current := &expr; current != nil; current = current.Left {
		if current.Kind != ir.ExprCall {
			continue
		}
		receiver, _, qualified := splitSemaMethodPath(current.Callee)
		if !qualified && current.Left != nil && current.Left.Kind == ir.ExprVariable {
			receiver, qualified = current.Left.Name, true
		}
		if !qualified {
			continue
		}
		root, _, _ := strings.Cut(receiver, ".")
		if _, scoped := scope.lookup(root); scoped {
			continue
		}
		if message := semaAutomationTypeMessage(receiver, model); message != "" {
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
	if message := semaAutomationTypeMessage(receiverType, model); message != "" {
		return message
	}
	if strings.EqualFold(receiverType, "FSL.ScheduleService") {
		return "Variable does not exist: FSL.ScheduleService"
	}
	var params [][]string
	canonicalType := receiverType
	switch normalizeName(receiverType) {
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
