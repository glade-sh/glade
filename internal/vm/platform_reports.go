package vm

import (
	"sort"
	"strings"
)

func reportsRuntimeDTOType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "reports.reportfilter", "reports.sortcolumn", "reports.groupinginfo",
		"reports.standarddatefilter", "reports.toprows", "reports.reporttype",
		"reports.reportmetadata", "reports.reportresults", "reports.reportdescriberesult",
		"reports.reportextendedmetadata", "reports.reporttypemetadata", "reports.reportfactwithdetails",
		"reports.reportfactwithsummaries", "reports.summaryvalue",
		"reports.detailcolumn", "reports.groupingcolumn":
		return true
	default:
		return false
	}
}

func reportsPassiveRuntimeClass(class Class) bool {
	if !reportsRuntimeDTOType(runtimeClassName(class)) || !passiveRuntimeClass(class) {
		return false
	}
	for _, method := range class.Methods {
		if passiveGeneratedMethod(method) {
			return true
		}
	}
	return false
}

// R153-R182/R185-R220: Reports getters expose private service DTO fields to
// typed JSON. Register that runtime shape without adding source-visible fields
// or setters. In particular, raw null collection fields stay null in JSON;
// their getters supply empty collections separately.
func initializeReportsRuntimeClass(class *Class) {
	if !reportsPassiveRuntimeClass(*class) {
		return
	}
	fields := make(map[string]Field, len(class.Fields))
	for name, field := range class.Fields {
		fields[name] = field
	}
	for _, method := range class.Methods {
		if method.IsStatic || len(method.Params) != 0 || !passiveGeneratedMethod(method) {
			continue
		}
		suffix, getter := passiveAccessorSuffix(apexMethodMemberName(method.Name), "get")
		if !getter {
			continue
		}
		name := strings.ToLower(suffix[:1]) + suffix[1:]
		if _, exists := fields[name]; !exists {
			fields[name] = Field{Name: name, Type: method.ReturnType, Access: "private", InitialValue: Null}
		}
	}
	class.Fields = fields
	class.FieldOrder = orderedFieldNames(fields, class.FieldOrder)
	sort.Strings(class.FieldOrder)
}

// R009/R014/R019: the three-string constructor selects fieldValue even when
// the strings are null or empty; the no-argument constructor leaves it null.
func initializeReportsConstructor(object *Value, args []Value) {
	if strings.EqualFold(object.Type, "reports.ReportFilter") && len(args) == 3 {
		object.Fields["filterType"] = reportsEnumStringValue("reports.ReportFilterType", String("fieldValue"))
	}
}

// R047-R050, R056-R068, R092-R095, R144-R149; K001-K006 confirm casing.
// These String overloads differ from enum.valueOf: an unknown name stores null.
func reportsEnumStringValue(typeName string, value Value) Value {
	if value.Kind != ValueString {
		return value
	}
	generated, ok := generatedPlatformTypes()[strings.ToLower(typeName)]
	if !ok {
		return Null
	}
	for i, name := range generatedPlatformEnumNames(generated) {
		if strings.EqualFold(name, value.Text) {
			return Value{Kind: ValueObject, Type: generated.Name, Text: name, Fields: map[string]Value{"ordinal": Int(int64(i))}}
		}
	}
	return Null
}

// Reports accessors share one implementation across linked methods and the
// unlinked platform fallback. JSON conversion remains in the JSON runtime.
func (vm *VM) reportsDTOAccessor(receiver Value, methodName string, args []Value) (Value, Value, bool, bool) {
	if receiver.Kind != ValueObject {
		return Null, receiver, false, false
	}
	typeName := strings.ToLower(receiver.Type)
	if !reportsRuntimeDTOType(typeName) {
		return Null, receiver, false, false
	}
	if class, registered := vm.lookupClass(receiver.Type); registered && !reportsPassiveRuntimeClass(class) {
		return Null, receiver, false, false
	}
	methodName = apexMethodMemberName(methodName)
	key := typeName + "." + strings.ToLower(methodName)
	if len(args) == 1 {
		enumType := ""
		switch key {
		case "reports.reportfilter.setfiltertype":
			enumType = "reports.ReportFilterType"
		case "reports.sortcolumn.setsortorder", "reports.groupinginfo.setsortorder", "reports.toprows.setdirection":
			enumType = "reports.ColumnSortOrder"
		case "reports.groupinginfo.setdategranularity":
			enumType = "reports.DateGranularity"
		case "reports.reportmetadata.setreportformat":
			enumType = "reports.ReportFormat"
		}
		if enumType != "" && (args[0].Kind == ValueString || args[0].Kind == ValueNull || strings.EqualFold(args[0].Type, enumType)) {
			suffix, _ := passiveAccessorSuffix(methodName, "set")
			field := passiveAccessorFieldName(receiver, suffix)
			receiver.Fields[field] = reportsEnumStringValue(enumType, args[0])
			return Null, receiver, true, true
		}
	}
	suffix, getter := passiveAccessorSuffix(methodName, "get")
	if !getter || len(args) != 0 {
		return Null, receiver, false, false
	}
	method, known := vm.generatedPlatformMethodForArgs(receiver.Type, methodName, nil, false)
	if !known {
		return Null, receiver, false, false
	}
	_, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix))
	if found && value.Kind != ValueNull {
		return value, receiver, false, true
	}
	// R137/R140/R143, R150-R152, R155/R162, R177-R182, R186-R196;
	// K007/K011-K014 distinguish empty getter collections from raw null fields.
	switch key {
	case "reports.reportmetadata.getaggregates", "reports.reportmetadata.getdetailcolumns",
		"reports.reportmetadata.gethistoricalsnapshotdates", "reports.reportmetadata.getreportfilters",
		"reports.reportmetadata.getgroupingsdown", "reports.reportmetadata.getsortby",
		"reports.reportfactwithdetails.getaggregates", "reports.reportfactwithdetails.getrows",
		"reports.reportfactwithsummaries.getaggregates":
		return typedList(method.ReturnType), receiver, false, true
	case "reports.reportresults.getfactmap", "reports.reportextendedmetadata.getdetailcolumninfo",
		"reports.reportextendedmetadata.getaggregatecolumninfo", "reports.reportextendedmetadata.getgroupingcolumninfo":
		return typedMap(method.ReturnType), receiver, false, true
	}
	// R153-R159 and K008-K010: missing scalar and nested DTO getters are null.
	return Null, receiver, false, true
}
