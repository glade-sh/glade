package vm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) callStandardControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	record, ok := receiver.Fields["record"]
	if !ok || record.Kind != ValueObject {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController has no SObject record")
	}
	switch method {
	case "getId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getId expects 0 arguments")
		}
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			return id, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getRecord expects 0 arguments")
		}
		return record, receiver, false, true, nil
	case "save", "quickSave":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		op := "insert"
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			if idText, ok := idValueText(id); ok && idText != "" {
				op = "update"
			}
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": op})
		results, err := vm.applyDML(op, record, true, "", dml.Options{}, result)
		if err != nil {
			appendStandardControllerErrorTrace(result, method, record, op, err)
			return Null, receiver, false, true, err
		}
		if len(results) > 0 && results[0].ID != "" {
			record.Fields["Id"] = String(string(results[0].ID))
			receiver.Fields["record"] = record
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  op,
			"pageReference": tracePageReference(page),
		})
		return page, receiver, true, true, nil
	case "delete":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.delete expects 0 arguments")
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": "delete"})
		if _, err := vm.applyDML("delete", record, true, "", dml.Options{}, result); err != nil {
			appendStandardControllerErrorTrace(result, method, record, "delete", err)
			return Null, receiver, false, true, err
		}
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  "delete",
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "view", "edit", "cancel", "reset":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "start", method, record, nil)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "addFields":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.addFields expects List")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexStackMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "empty", "peek", "pop", "push")
	values := receiver.Fields["values"]
	if values.Kind != ValueList {
		values = List()
	}
	switch method {
	case "empty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.empty expects 0 arguments")
		}
		return Bool(len(values.List) == 0), receiver, false, true, nil
	case "peek":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.peek expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		return values.List[len(values.List)-1], receiver, false, true, nil
	case "pop":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.pop expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		value := values.List[len(values.List)-1]
		values.List = values.List[:len(values.List)-1]
		receiver.Fields["values"] = values
		return value, receiver, true, true, nil
	case "push":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.push expects 1 argument")
		}
		values.List = append(values.List, args[0])
		receiver.Fields["values"] = values
		return args[0], receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesActionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getExpression", "invoke")
	switch method {
	case "getExpression":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.getExpression expects 0 arguments")
		}
		if value, ok := receiver.Fields["expression"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "invoke":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.invoke expects 0 arguments")
		}
		expression := ""
		if value, ok := receiver.Fields["expression"]; ok && value.Kind == ValueString {
			expression = strings.TrimSpace(value.Text)
		}
		if expression == "" || strings.EqualFold(expression, "null") || strings.EqualFold(expression, "{!null}") || strings.EqualFold(expression, "{!}") {
			return Null, receiver, false, true, nil
		}
		if strings.EqualFold(expression, "list") || strings.EqualFold(expression, "{!list}") {
			return newPageReference("/list"), receiver, false, true, nil
		}
		return Null, receiver, false, true, unsupportedCallError("ApexPages.Action.invoke requires bound Visualforce controller lifecycle")
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "withFormula", "withReturnType", "withType", "withGlobalVariables", "treatNumericNullAsZero", "parseAsTemplate", "build")
	switch method {
	case "withFormula":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withFormula expects formula String")
		}
		receiver.Fields["formula"] = args[0]
		return receiver, receiver, true, true, nil
	case "withReturnType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withReturnType expects return type")
		}
		receiver.Fields["returnType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withType expects context type")
		}
		receiver.Fields["contextType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withGlobalVariables":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withGlobalVariables expects FormulaGlobal list")
		}
		receiver.Fields["globalVariables"] = args[0]
		return receiver, receiver, true, true, nil
	case "treatNumericNullAsZero":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.treatNumericNullAsZero expects Boolean")
		}
		receiver.Fields["treatNumericNullAsZero"] = args[0]
		return receiver, receiver, true, true, nil
	case "parseAsTemplate":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.parseAsTemplate expects Boolean")
		}
		receiver.Fields["templateMode"] = args[0]
		return receiver, receiver, true, true, nil
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.build expects 0 arguments")
		}
		if value, ok := receiver.Fields["templateMode"]; ok && value.Kind == ValueBool && value.Bool {
			return Null, receiver, false, true, unsupportedCallError("formulaeval.FormulaBuilder.parseAsTemplate template evaluation")
		}
		instance := Object("formulaeval.FormulaInstance")
		for field, value := range receiver.Fields {
			instance.Fields[field] = value
		}
		return instance, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaInstanceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "evaluate", "getReferencedFields")
	switch method {
	case "evaluate":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.evaluate expects context object")
		}
		formula, _ := formulaInstanceText(receiver)
		if formula == "" {
			return Null, receiver, false, true, newExceptionError("FormulaEvaluationException", "formula text is required")
		}
		value, ok := vm.evaluateFormulaInstanceValue(receiver, args[0], formula)
		if !ok {
			return Null, receiver, false, true, unsupportedCallError("formulaeval.FormulaInstance.evaluate unsupported local formula expression")
		}
		return value, receiver, false, true, nil
	case "getReferencedFields":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.getReferencedFields expects 0 arguments")
		}
		formula, _ := formulaInstanceText(receiver)
		out := Set()
		out.Type = "Set<String>"
		for _, field := range formulaReferencedFields(formula) {
			out.Set = append(out.Set, String(field))
		}
		return out, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "isSuccess", "getSObject", "getErrors")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "isSuccess":
		if value, ok := receiver.Fields["success"]; ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case "getSObject":
		if value, ok := receiver.Fields["sobject"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getErrors":
		if value, ok := receiver.Fields["errors"]; ok {
			return value, receiver, false, true, nil
		}
		return typedList("List<FormulaRecalcFieldError>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcFieldErrorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getFieldName", "getFieldError")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "getFieldName":
		if value, ok := receiver.Fields["fieldName"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getFieldError":
		if value, ok := receiver.Fields["fieldError"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) recalculateFormulaList(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
	}
	out := typedList("List<FormulaRecalcResult>")
	for _, item := range args[0].List {
		if item.Kind != ValueObject || !vm.isSObjectLikeType(item.Type) {
			return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
		}
		out.List = append(out.List, vm.recalculateFormulaSObject(item))
	}
	return out, nil
}

func (vm *VM) recalculateFormulaSObject(item Value) Value {
	result := Object("FormulaRecalcResult")
	result.Fields["sobject"] = item
	result.Fields["success"] = Bool(true)
	errors := typedList("List<FormulaRecalcFieldError>")
	if vm.Org == nil {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "org metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	objectName, ok := vm.resolveObjectName(item.Type)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "object metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	definition := vm.Org.Objects[objectName].Definition
	record, ok := vm.formulaRecordFromSObject(item)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "SObject value cannot be converted to a formula context"))
		result.Fields["errors"] = errors
		return result
	}
	for fieldName, field := range definition.Fields {
		if field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrg(field.Formula, field, vm.Org, definition, record)
		if !ok {
			result.Fields["success"] = Bool(false)
			errors.List = append(errors.List, formulaRecalcFieldError(fieldName, "unsupported local formula expression"))
			continue
		}
		if explicitNull {
			item.Fields[fieldName] = Null
			continue
		}
		item.Fields[fieldName] = vmValueFromStorage(value)
	}
	result.Fields["sobject"] = item
	result.Fields["errors"] = errors
	return result
}

func formulaRecalcFieldError(fieldName, message string) Value {
	err := Object("FormulaRecalcFieldError")
	err.Fields["fieldName"] = String(fieldName)
	err.Fields["fieldError"] = String(message)
	return err
}

func (vm *VM) evaluateFormulaInstanceValue(instance Value, context Value, formula string) (Value, bool) {
	if context.Kind != ValueObject || !vm.isSObjectLikeType(context.Type) || vm.Org == nil {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(context.Type)
	if !ok {
		return Null, false
	}
	definition := vm.Org.Objects[objectName].Definition
	record, ok := vm.formulaRecordFromSObject(context)
	if !ok {
		return Null, false
	}
	field := storage.Field{APIName: "__formula", Type: formulaReturnFieldType(instance), Formula: formula}
	value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrg(formula, field, vm.Org, definition, record)
	if !ok {
		return Null, false
	}
	if explicitNull {
		return Null, true
	}
	return vmValueFromStorage(value), true
}

func formulaInstanceText(instance Value) (string, bool) {
	if value, ok := instance.Fields["formula"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	if value, ok := instance.Fields["formulaText"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	return "", false
}

func formulaReturnFieldType(instance Value) storage.FieldType {
	if value, ok := instance.Fields["returnType"]; ok {
		name := strings.ToUpper(strings.TrimSpace(value.Text))
		if name == "" && value.Kind == ValueObject {
			name = strings.ToUpper(strings.TrimSpace(value.Text))
		}
		switch name {
		case "BOOLEAN":
			return storage.FieldBoolean
		case "INTEGER", "LONG":
			return storage.FieldInteger
		case "DECIMAL", "DOUBLE":
			return storage.FieldDecimal
		case "DATE":
			return storage.FieldDate
		case "DATETIME":
			return storage.FieldDateTime
		case "ID":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func formulaReferencedFields(formula string) []string {
	matches := regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:__c|__r)?(?:\.[A-Za-z_][A-Za-z0-9_]*(?:__c|__r)?)*\b`).FindAllString(formula, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		upper := strings.ToUpper(match)
		switch upper {
		case "AND", "OR", "NOT", "IF", "CASE", "ISBLANK", "ISNULL", "NULL", "TRUE", "FALSE", "TODAY", "NOW", "DATE", "DATETIMEVALUE", "TEXT", "VALUE", "LOWER", "UPPER", "FLOOR", "MOD", "REGEX", "CONTAINS":
			continue
		}
		if seen[strings.ToLower(match)] {
			continue
		}
		seen[strings.ToLower(match)] = true
		out = append(out, match)
	}
	sort.Strings(out)
	return out
}

func callContinuationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "addHttpRequest", "getRequests")
	switch method {
	case "addHttpRequest":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
			return Null, receiver, false, true, fmt.Errorf("Continuation.addHttpRequest expects HttpRequest")
		}
		requests, ok := receiver.Fields["requests"]
		if !ok || requests.Kind != ValueMap {
			requests = typedMap("Map<String,HttpRequest>")
		}
		label := fmt.Sprintf("request-%d", len(requests.Map)+1)
		key := mapKey(String(label))
		requests.Map[key] = args[0]
		requests.MapKeys[key] = String(label)
		receiver.Fields["requests"] = requests
		return String(label), receiver, true, true, nil
	case "getRequests":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Continuation.getRequests expects 0 arguments")
		}
		if requests, ok := receiver.Fields["requests"]; ok && requests.Kind == ValueMap {
			return requests, receiver, false, true, nil
		}
		return typedMap("Map<String,HttpRequest>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesComponentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getComponentById")
	if method != "getComponentById" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, receiver, false, true, fmt.Errorf("%s.getComponentById expects id String", receiver.Type)
	}
	return Null, receiver, false, true, nil
}

func callApexPagesIdeaStandardControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getCommentList")
	if method != "getCommentList" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardController.getCommentList expects 0 arguments")
	}
	return typedList("List<IdeaComment>"), receiver, false, true, nil
}

func callApexPagesIdeaStandardSetControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getIdeaList", "getListViewOptions")
	switch method {
	case "getIdeaList":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getIdeaList expects 0 arguments")
		}
		return typedList("List<Idea>"), receiver, false, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getListViewOptions expects 0 arguments")
		}
		return typedList("List<SelectOption>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesKnowledgeArticleVersionStandardControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getSourceId", "selectDataCategory")
	switch method {
	case "getSourceId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.getSourceId expects 0 arguments")
		}
		return Null, receiver, false, true, nil
	case "selectDataCategory":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.selectDataCategory expects group and category Strings")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func appendStandardControllerActionTrace(result *Result, phase, method string, record Value, extra map[string]any) {
	args := standardControllerTraceArgs(method, record)
	for key, value := range extra {
		args[key] = value
	}
	appendTrace(result, "apex.visualforce.standard_controller.action."+phase, "apex.visualforce.standard_controller", args)
}

func appendStandardControllerErrorTrace(result *Result, method string, record Value, dmlOperation string, err error) {
	actionErr := uiInvocationError(err)
	appendStandardControllerActionTrace(result, "error", method, record, map[string]any{
		"dmlOperation": dmlOperation,
		"error":        actionErr.Message,
		"errorType":    actionErr.Type,
	})
}

func standardControllerTraceArgs(method string, record Value) map[string]any {
	args := map[string]any{"method": method}
	if record.Kind == ValueObject {
		args["objectType"] = record.Type
		if _, id, ok := objectFieldValue(record, "Id"); ok && id.Kind == ValueString && id.Text != "" {
			args["recordId"] = id.Text
		}
	}
	return args
}

func (vm *VM) callStandardSetControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	records := receiver.Fields["records"]
	switch method {
	case "getRecords":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecords expects 0 arguments")
		}
		return standardSetCurrentPage(receiver, records), receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecord expects 0 arguments")
		}
		page := standardSetCurrentPage(receiver, records)
		if len(page.List) == 0 {
			return Null, receiver, false, true, nil
		}
		return page.List[0], receiver, false, true, nil
	case "getResultSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getResultSize expects 0 arguments")
		}
		if records.Kind != ValueList {
			return Int(0), receiver, false, true, nil
		}
		return Int(int64(len(records.List))), receiver, false, true, nil
	case "getSelected":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getSelected expects 0 arguments")
		}
		return receiver.Fields["selected"], receiver, false, true, nil
	case "setSelected":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setSelected expects List")
		}
		receiver.Fields["selected"] = args[0]
		return Null, receiver, true, true, nil
	case "getPageSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageSize expects 0 arguments")
		}
		return receiver.Fields["pageSize"], receiver, false, true, nil
	case "setPageSize":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageSize expects positive Integer")
		}
		receiver.Fields["pageSize"] = args[0]
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "getPageNumber":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageNumber expects 0 arguments")
		}
		return receiver.Fields["pageNumber"], receiver, false, true, nil
	case "setPageNumber":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageNumber expects positive Integer")
		}
		page := int(args[0].Int)
		pageCount := standardSetPageCount(receiver, records)
		if page > pageCount {
			page = pageCount
		}
		receiver.Fields["pageNumber"] = Int(int64(page))
		return Null, receiver, true, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getListViewOptions expects 0 arguments")
		}
		return typedList("List<SelectOption>"), receiver, false, true, nil
	case "setFilterId":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setFilterId expects String")
		}
		receiver.Fields["filterId"] = args[0]
		return Null, receiver, true, true, nil
	case "getFilterId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getFilterId expects 0 arguments")
		}
		if value, ok := receiver.Fields["filterId"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "first":
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "last":
		receiver.Fields["pageNumber"] = Int(int64(standardSetPageCount(receiver, records)))
		return Null, receiver, true, true, nil
	case "next":
		page := int(receiver.Fields["pageNumber"].Int)
		if page < standardSetPageCount(receiver, records) {
			receiver.Fields["pageNumber"] = Int(int64(page + 1))
		}
		return Null, receiver, true, true, nil
	case "previous":
		page := int(receiver.Fields["pageNumber"].Int)
		if page > 1 {
			receiver.Fields["pageNumber"] = Int(int64(page - 1))
		}
		return Null, receiver, true, true, nil
	case "getHasNext":
		return Bool(int(receiver.Fields["pageNumber"].Int) < standardSetPageCount(receiver, records)), receiver, false, true, nil
	case "getHasPrevious":
		return Bool(receiver.Fields["pageNumber"].Int > 1), receiver, false, true, nil
	case "getCompleteResult":
		return Bool(true), receiver, false, true, nil
	case "save":
		return vm.standardSetDML(receiver, "update", result)
	case "delete":
		return vm.standardSetDML(receiver, "delete", result)
	case "cancel":
		return newPageReference(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
