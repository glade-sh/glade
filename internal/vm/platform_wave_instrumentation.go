package vm

import (
	"fmt"
	"strings"
)

func callWaveQueryBuilderStaticDefault(method string, args []Value) (Value, bool) {
	switch strings.ToLower(method) {
	case "load":
		if len(args) != 2 {
			return Null, false
		}
		node := newWaveQueryNode("load")
		node.Fields["datasetId"] = args[0]
		node.Fields["datasetVersionId"] = args[1]
		return node, true
	case "loadbydevelopername":
		if len(args) != 1 {
			return Null, false
		}
		node := newWaveQueryNode("load")
		node.Fields["developerName"] = args[0]
		return node, true
	case "union":
		if len(args) != 1 {
			return Null, false
		}
		node := newWaveQueryNode("union")
		node.Fields["nodes"] = args[0]
		return node, true
	case "cogroup":
		if len(args) != 2 {
			return Null, false
		}
		node := newWaveQueryNode("cogroup")
		node.Fields["nodes"] = args[0]
		node.Fields["groups"] = args[1]
		return node, true
	case "count":
		if len(args) != 0 {
			return Null, false
		}
		projection := newWaveProjectionNode("count")
		return projection, true
	case "get":
		if len(args) != 1 {
			return Null, false
		}
		projection := newWaveProjectionNode("get")
		projection.Fields["projection"] = args[0]
		return projection, true
	default:
		return Null, false
	}
}

func callWaveQueryMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "wave.QueryNode":
		return callWaveQueryNodeMember(receiver, method, args)
	case "wave.ProjectionNode":
		return callWaveProjectionNodeMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callWaveQueryNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "build":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.build expects String streamName")
		}
		return String(waveQueryNodeBuild(receiver, args[0].Text)), receiver, false, true, nil
	case "execute":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.execute expects String streamName")
		}
		out := Object("ConnectApi.LiteralJson")
		out.Fields["json"] = typedList("List<Object>")
		return out, receiver, false, true, nil
	case "cap", "filter", "foreach", "group", "order":
		if strings.EqualFold(method, "group") && len(args) > 1 {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.group expects 0 or 1 argument")
		}
		if !strings.EqualFold(method, "group") && len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.%s expects 1 argument", method)
		}
		waveAppendStep(&receiver, method, args)
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callWaveProjectionNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	name := strings.ToLower(method)
	switch name {
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.build expects 0 arguments")
		}
		return String(waveProjectionNodeBuild(receiver)), receiver, false, true, nil
	case "alias":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.alias expects String")
		}
		receiver.Fields["alias"] = args[0]
		return receiver, receiver, true, true, nil
	case "avg", "count", "max", "min", "sum", "unique":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.%s expects 0 arguments", method)
		}
		receiver.Fields["aggregate"] = String(name)
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func newWaveQueryNode(kind string) Value {
	node := Object("wave.QueryNode")
	node.Fields["kind"] = String(kind)
	node.Fields["steps"] = typedList("List<String>")
	return node
}

func newWaveProjectionNode(kind string) Value {
	projection := Object("wave.ProjectionNode")
	projection.Fields["kind"] = String(kind)
	return projection
}

func waveAppendStep(receiver *Value, method string, args []Value) {
	steps := typedList("List<String>")
	if _, existing, ok := objectFieldValue(*receiver, "steps"); ok && existing.Kind == ValueList {
		steps = existing
	}
	step := strings.ToLower(method)
	if len(args) > 0 {
		step += "(" + args[0].String() + ")"
	} else {
		step += "()"
	}
	steps.List = append(steps.List, String(step))
	receiver.Fields["steps"] = steps
}

func waveQueryNodeBuild(node Value, streamName string) string {
	parts := []string{streamName}
	if _, kind, ok := objectFieldValue(node, "kind"); ok && kind.Kind == ValueString && kind.Text != "" {
		parts = append(parts, kind.Text)
	}
	if _, value, ok := objectFieldValue(node, "developerName"); ok && value.Kind == ValueString {
		parts = append(parts, value.Text)
	} else if _, value, ok := objectFieldValue(node, "datasetId"); ok && value.Kind == ValueString {
		parts = append(parts, value.Text)
	}
	if _, steps, ok := objectFieldValue(node, "steps"); ok && steps.Kind == ValueList {
		for _, step := range steps.List {
			if step.Kind == ValueString {
				parts = append(parts, step.Text)
			}
		}
	}
	return strings.Join(parts, " | ")
}

func waveProjectionNodeBuild(projection Value) string {
	kind := "projection"
	if _, value, ok := objectFieldValue(projection, "kind"); ok && value.Kind == ValueString && value.Text != "" {
		kind = value.Text
	}
	if _, value, ok := objectFieldValue(projection, "projection"); ok && value.Kind == ValueString && value.Text != "" {
		kind = value.Text
	}
	if _, aggregate, ok := objectFieldValue(projection, "aggregate"); ok && aggregate.Kind == ValueString && aggregate.Text != "" {
		kind = aggregate.Text + "(" + kind + ")"
	}
	if _, alias, ok := objectFieldValue(projection, "alias"); ok && alias.Kind == ValueString && alias.Text != "" {
		kind += " as " + alias.Text
	}
	return kind
}

func callContextIndustriesContextMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "Context.IndustriesContext") {
		return Null, receiver, false, false, nil
	}
	name := strings.ToLower(method)
	if len(args) != 1 || args[0].Kind != ValueMap {
		return Null, receiver, false, true, fmt.Errorf("Context.IndustriesContext.%s expects Map<String,Object>", method)
	}
	switch name {
	case "deletecontext", "evictcontextdefinition":
		return Null, receiver, false, true, nil
	case "addrecordstocontext", "buildcontext", "filteringcontext", "getcontext",
		"getcontexttranslation", "leanerquerytags", "persistcontext",
		"querycontextrecordsandchildren", "queryrecordstatus", "querytags",
		"updatecontextattributes":
		return args[0], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationOperationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationOperation") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "createnewspan":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.createNewSpan expects 0 arguments")
		}
		return Object("TracerSpan"), receiver, false, true, nil
	case "start":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.start expects publish type")
		}
		context := Object("OrgInstrumentationContext")
		context.Fields["publishType"] = args[0]
		context.Fields["started"] = Bool(true)
		context.Fields["duration"] = Int(0)
		return context, receiver, false, true, nil
	case "end":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.end expects context")
		}
		return Null, receiver, false, true, nil
	case "endwithstatus":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.endWithStatus expects context and status code")
		}
		return Null, receiver, false, true, nil
	case "setmetrictags":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.setMetricTags expects Map<String,String>")
		}
		receiver.Fields["metricTags"] = args[0]
		return Null, receiver, true, true, nil
	case "publishcustomhistogramvalues", "publishcustomincrementalvalue", "publishcustompercentileset",
		"publishincrementalvalue", "publishpercentileset", "publishrequestcountandduration":
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationContextMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationContext") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "starttime":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.startTime expects 0 arguments")
		}
		receiver.Fields["started"] = Bool(true)
		receiver.Fields["duration"] = Int(0)
		return Null, receiver, true, true, nil
	case "end":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.end expects 0 arguments")
		}
		receiver.Fields["ended"] = Bool(true)
		return Null, receiver, true, true, nil
	case "getduration":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.getDuration expects 0 arguments")
		}
		if value, ok := receiver.Fields["duration"]; ok {
			return value, receiver, false, true, nil
		}
		return Int(0), receiver, false, true, nil
	case "getpublishtype":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.getPublishType expects 0 arguments")
		}
		if value, ok := receiver.Fields["publishType"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationServiceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationService") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "getinstrumentationoperation":
		if len(args) != 2 && len(args) != 3 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.getInstrumentationOperation expects name, tags[, buckets]")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.getInstrumentationOperation expects String and Map<String,String>")
		}
		if len(args) == 3 && args[2].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.getInstrumentationOperation buckets expects List")
		}
		operation := Object("OrgInstrumentationOperation")
		operation.Fields["operationName"] = args[0]
		operation.Fields["metricTags"] = args[1]
		if len(args) == 3 {
			operation.Fields["buckets"] = args[2]
		}
		return operation, receiver, false, true, nil
	case "gettracercontext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.getTracerContext expects 0 arguments")
		}
		return typedMap("Map<String,String>"), receiver, false, true, nil
	case "propagatecontext":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.propagateContext expects HttpRequest")
		}
		httpSetHeader(args[0], "x-glade-instrumentation-context", String("local"))
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callUserProvisioningBatchableMember(vm *VM, receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !vm.isUserProvisioningBatchableType(receiver.Type) {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "start":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.start expects BatchableContext", receiver.Type)
		}
		locator := Object("Database.QueryLocator")
		locator.Fields["Records"] = typedList("List<SObject>")
		locator.Fields["Query"] = String("")
		return locator, receiver, false, true, nil
	case "execute":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("%s.execute expects BatchableContext and scope", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "finish":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.finish expects BatchableContext", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "flowinputpreprocessing":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("%s.flowInputPreprocessing expects Map<String,Object>", receiver.Type)
		}
		return args[0], receiver, false, true, nil
	case "flowpostprocessing":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("%s.flowPostProcessing expects handler output and SObject", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "postbatchprocessing":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.postBatchProcessing expects 0 arguments", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "geteventprefix", "getflowname", "getflownamespace":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		return String(""), receiver, false, true, nil
	case "getperbatchupl":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getPerBatchUPL expects 0 arguments", receiver.Type)
		}
		return typedList("List<SObject>"), receiver, false, true, nil
	case "getperbatchupr":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getPerBatchUPR expects 0 arguments", receiver.Type)
		}
		return typedList("List<UserProvisioningRequest>"), receiver, false, true, nil
	case "getuprtonewuplmap":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getUprToNewUplMap expects 0 arguments", receiver.Type)
		}
		return typedMap("Map<Id,SObject>"), receiver, false, true, nil
	case "hasflow", "hasfloworapex":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		return Bool(false), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) isUserProvisioningBatchableType(typeName string) bool {
	if userProvisioningBatchableType(typeName) {
		return true
	}
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return false
	}
	seen := map[string]bool{}
	for current := class; ; {
		name := runtimeClassName(current)
		key := strings.ToLower(strings.TrimSpace(name))
		if key != "" && seen[key] {
			return false
		}
		if key != "" {
			seen[key] = true
		}
		if userProvisioningBatchableType(name) {
			return true
		}
		if strings.TrimSpace(current.SuperClass) == "" {
			return false
		}
		next, ok := vm.lookupClass(current.SuperClass)
		if !ok {
			return userProvisioningBatchableType(current.SuperClass)
		}
		current = next
	}
}

func userProvisioningBatchableType(typeName string) bool {
	switch typeName {
	case "UserProvisioning.ProvisioningBatchable", "UserProvisioning.CollectingBatchable",
		"UserProvisioning.PluginBatchable", "UserProvisioning.LinkingBatchable",
		"UserProvisioning.CommittingBatchable", "UserProvisioning.DeletingBatchable",
		"UserProvisioning.RequestingBatchable", "UserProvisioning.UPASCleaningBatchable":
		return true
	default:
		return false
	}
}

func callPlatformCallbackDefaultMember(receiver Value, method string, args []Value) (Value, bool, error) {
	typeName := receiver.Type
	lowerMethod := strings.ToLower(method)
	switch typeName {
	case "workflow.Action":
		if lowerMethod != "invoke" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("workflow.Action.invoke expects Context")
		}
		return typedList("List<workflow.ActionDml>"), true, nil
	case "workflow.ActionDml":
		if lowerMethod != "invoke" {
			return Null, false, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("workflow.ActionDml.invoke expects 0 arguments")
		}
		return Null, true, nil
	case "eventbus.EventPublishFailureCallback":
		if lowerMethod != "onfailure" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("eventbus.EventPublishFailureCallback.onFailure expects FailureResult")
		}
		return Null, true, nil
	case "eventbus.EventPublishSuccessCallback":
		if lowerMethod != "onsuccess" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("eventbus.EventPublishSuccessCallback.onSuccess expects SuccessResult")
		}
		return Null, true, nil
	case "TxnSecurity.EventCondition", "TxnSecurity.PolicyCondition":
		if lowerMethod != "evaluate" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.evaluate expects event input", typeName)
		}
		return Bool(false), true, nil
	case "Social.DefaultInboundSocialPostHandler", "Social.InboundSocialPostHandlerImpl":
		return socialInboundHandlerDefault(typeName, lowerMethod, args)
	case "Social.InboundSocialPostHandler":
		if lowerMethod != "handleinboundsocialpost" {
			return Null, false, nil
		}
		if len(args) != 3 {
			return Null, true, fmt.Errorf("Social.InboundSocialPostHandler.handleInboundSocialPost expects post, persona, and rawData")
		}
		return Object("Social.InboundSocialPostResult"), true, nil
	default:
		return Null, false, nil
	}
}

func socialInboundHandlerDefault(typeName, method string, args []Value) (Value, bool, error) {
	switch method {
	case "createpersonaparent":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.createPersonaParent expects SocialPersona", typeName)
		}
		return Null, true, nil
	case "getcasesubject", "getpersonafirstname", "getpersonalastname":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.%s expects one argument", typeName, method)
		}
		return String(""), true, nil
	case "getdefaultaccountid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getDefaultAccountId expects 0 arguments", typeName)
		}
		return String(""), true, nil
	case "getmaxnumberofdaysclosedtoreopencase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getMaxNumberOfDaysClosedToReopenCase expects 0 arguments", typeName)
		}
		return Int(0), true, nil
	case "getposttagsthatcreatecase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getPostTagsThatCreateCase expects 0 arguments", typeName)
		}
		return typedSet("Set<String>"), true, nil
	case "getusingcaseassignmentrule":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getUsingCaseAssignmentRule expects 0 arguments", typeName)
		}
		return Bool(false), true, nil
	case "handleinboundsocialpost":
		if len(args) != 3 {
			return Null, true, fmt.Errorf("%s.handleInboundSocialPost expects post, persona, and rawData", typeName)
		}
		return Object("Social.InboundSocialPostResult"), true, nil
	default:
		return Null, false, nil
	}
}
