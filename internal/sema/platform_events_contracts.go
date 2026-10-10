package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// Captured contracts apply only to the resolved platform event surface.
func semaPlatformEventType(name string, model *semaTypeMemberView) bool {
	members, _, ok := semaLookupTypeMembers(model, name)
	return ok && members.sobject && members.platformEvent
}

func semaPlatformEventConstructorMessage(name string, args []string, namedCount int, model *semaTypeMemberView) string {
	if namedCount != 0 || !semaDMLPlatformType(model, name) {
		return ""
	}
	canonical := strings.ToLower(name)
	if canonical == "eventbus.retryableexception" || canonical == "eventbus.invalidreplayidexception" {
		// K017/K022 use a literal null, unlike the typed String null in R203.
		if len(args) == 1 && strings.EqualFold(args[0], "null") {
			return "Ambiguous method signature: void <init>(NULL)"
		}
		if canonical == "eventbus.retryableexception" && len(args) == 1 && strings.EqualFold(args[0], "Integer") {
			return "Constructor not defined: [eventbus.RetryableException].<Constructor>(Integer)"
		}
	}
	if len(args) != 0 {
		return ""
	}
	switch canonical {
	case "eventbus.triggercontext":
		return "Constructor not defined: [eventbus.TriggerContext].<Constructor>()"
	case "eventbus.successresult":
		return "Type cannot be constructed: eventbus.SuccessResult"
	case "eventbus.failureresult":
		return "Type cannot be constructed: eventbus.FailureResult"
	}
	return ""
}

func semaPlatformEventWriteMessage(receiver, field string, model *semaTypeMemberView) string {
	if strings.EqualFold(receiver, "eventbus.TriggerContext") && strings.EqualFold(field, "retries") && semaDMLPlatformType(model, receiver) {
		return "Variable is not visible: eventbus.TriggerContext.retries"
	}
	if semaPlatformEventType(receiver, model) {
		for _, name := range []string{"EventUuid", "ReplayId", "CreatedDate"} {
			if strings.EqualFold(field, name) {
				return "Field is not writeable: " + receiver + "." + name
			}
		}
	}
	return ""
}

func semaPlatformEventCallMessage(receiver, method, mode string, args []string, model *semaTypeMemberView) string {
	if !semaDMLPlatformType(model, receiver) {
		return ""
	}
	rejected := false
	display := receiver
	switch strings.ToLower(semaCanonicalPlatformAlias(receiver)) {
	case "eventbus":
		if !strings.EqualFold(method, "publish") {
			return ""
		}
		display = "EventBus"
		rejected = len(args) < 1 || len(args) > 2
		if !rejected {
			arg := args[0]
			base, elements := semaGenericBaseAndArgs(arg)
			recordType := arg
			if strings.EqualFold(base, "List") && len(elements) == 1 {
				recordType = elements[0]
			}
			// P001-P003: the standard event is not directly publishable.
			if strings.EqualFold(recordType, "BatchApexErrorEvent") && semaDMLRecordType(recordType, model) {
				return "DML operation Insert not allowed on BatchApexErrorEvent"
			}
			if semaDMLRecordType(arg, model) {
				if !strings.EqualFold(arg, "SObject") && !semaPlatformEventType(arg, model) {
					return "Argument must be a Platform Event sObject type."
				}
			} else if !strings.EqualFold(arg, "null") && !(strings.EqualFold(base, "List") && len(elements) == 1 && semaDMLRecordType(elements[0], model)) {
				rejected = true
			}
		}
	case "eventbus.triggercontext":
		display = "eventbus.TriggerContext"
		// R198-R202: preserve the native type-specific signature diagnostic.
		rejected = strings.EqualFold(method, "setResumeCheckpoint") && !semaArgsMatchAny([][]string{{"String"}}, args, model)
	case "eventbus.testbroker":
		display = "eventbus.TestBroker"
		if strings.EqualFold(method, "deliver") && mode == "class" && len(args) == 0 {
			return "Non static method cannot be referenced from a static context: void eventbus.TestBroker.deliver()"
		}
		rejected = strings.EqualFold(method, "deliver") && len(args) != 0
	case "eventbus.failureresult":
		display = "eventbus.FailureResult"
		rejected = strings.EqualFold(method, "isSuccess")
	}
	if !rejected {
		return ""
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(args, ","), display)
}

func semaPlatformEventDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA028", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, end)}
}

// C024/C032: a rejected EventBus call precedes a derivative void-assignment error.
func semaPlatformEventPrioritizeCallDiagnostics(items, irItems []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	for _, call := range irItems {
		if call.Code != "GLADESEMA028" || call.Range == nil {
			continue
		}
		// GLADESEMA028 is shared with async and HTTP. Only the captured
		// EventBus calls take precedence over a derivative void assignment.
		ownedCall := call.NativeMessage == "Non static method cannot be referenced from a static context: void eventbus.TestBroker.deliver()"
		if strings.HasPrefix(call.NativeMessage, "Method does not exist or incorrect signature: void isSuccess(") {
			_, receiver, qualified := strings.Cut(call.NativeMessage, " from the type ")
			ownedCall = qualified && receiver == "eventbus.FailureResult"
		}
		if !ownedCall {
			continue
		}
		for i, item := range items {
			if item.Code == "GLADESEMA018" && strings.HasSuffix(item.Message, "with void") && item.File == call.File && item.Range != nil &&
				item.Range.Start.Offset < call.Range.End.Offset && call.Range.Start.Offset < item.Range.End.Offset {
				items[i] = call
			}
		}
	}
	return items
}
