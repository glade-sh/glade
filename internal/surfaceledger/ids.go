package surfaceledger

import (
	"strings"
	"unicode"
)

func ApexTypeID(namespace, typeName string) string {
	namespace, typeName = canonicalApexQualifiedParts(namespace, typeName)
	return "apex:" + qualifiedName(namespace, typeName)
}

func ApexMemberID(namespace, typeName, memberName string, parameters []string) string {
	namespace, typeName = canonicalApexQualifiedParts(namespace, typeName)
	id := ApexTypeID(namespace, typeName) + "." + canonicalApexMemberName(memberName)
	if parameters != nil {
		id += "(" + strings.Join(cleanList(parameters), ",") + ")"
	}
	return id
}

func ToolingObjectID(objectName string) string {
	return "tooling:" + strings.TrimSpace(objectName)
}

func ToolingFieldID(objectName, fieldName string) string {
	return ToolingObjectID(objectName) + "." + strings.TrimSpace(fieldName)
}

func RestResourceID(resource, method string) string {
	return "rest:" + strings.Trim(strings.TrimSpace(resource), "/") + "." + strings.ToLower(strings.TrimSpace(method))
}

func VisualforceAttrID(namespace, component, attr string) string {
	return "visualforce:" + strings.TrimSpace(namespace) + ":" + strings.TrimSpace(component) + "." + strings.TrimSpace(attr)
}

func LWCModuleID(module string) string {
	return "lwc:" + strings.TrimSpace(module)
}

func AuraID(path string) string {
	path = strings.TrimSuffix(strings.TrimSpace(path), ".md")
	path = strings.ReplaceAll(path, "/", ".")
	return "aura:" + path
}

func qualifiedName(namespace, typeName string) string {
	namespace = cleanIdentityPart(namespace)
	typeName = cleanIdentityPart(typeName)
	if namespace == "" {
		return typeName
	}
	if strings.HasPrefix(typeName, namespace+".") {
		return typeName
	}
	return namespace + "." + typeName
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, canonicalParameterType(value))
		}
	}
	return out
}

func canonicalParameterType(value string) string {
	value = cleanIdentityPart(value)
	switch strings.ToUpper(value) {
	case "APEX_OBJECT":
		return "Object"
	case "ANY":
		return "Object"
	case "SOBJECT":
		return "Object"
	case "SOBJECT[]":
		return "List<Object>"
	case "SYSTEM.TYPE":
		return "Type"
	default:
		value = strings.ReplaceAll(value, "<ANY>", "<Object>")
		value = strings.ReplaceAll(value, "<sObject>", "<Object>")
		value = strings.ReplaceAll(value, "<SObject>", "<Object>")
		value = strings.ReplaceAll(value, "cache.", "Cache.")
		value = strings.ReplaceAll(value, ",ANY>", ",Object>")
		value = strings.ReplaceAll(value, ", ANY>", ", Object>")
		value = strings.ReplaceAll(value, ",sObject>", ",Object>")
		value = strings.ReplaceAll(value, ", sObject>", ", Object>")
		value = strings.ReplaceAll(value, ",SObject>", ",Object>")
		value = strings.ReplaceAll(value, ", SObject>", ", Object>")
		return value
	}
}

func canonicalApexQualifiedParts(namespace, typeName string) (string, string) {
	namespace = canonicalApexNamespaceName(namespace)
	typeName = cleanIdentityPart(typeName)
	if namespace == "ApexPages" && typeName == "ApexPages" {
		namespace = "System"
	}
	if namespace == "System" {
		switch typeName {
		case "ChildRelationship", "DescribeFieldResult", "DescribeSObjectResult", "PicklistEntry", "RecordTypeInfo", "SObjectField", "SObjectType":
			namespace = "Schema"
		}
	}
	return namespace, typeName
}

func canonicalApexNamespaceName(namespace string) string {
	namespace = cleanIdentityPart(namespace)
	switch strings.ToLower(namespace) {
	case "cache":
		return "Cache"
	case "connectapi":
		return "ConnectApi"
	case "database":
		return "Database"
	case "schema":
		return "Schema"
	case "system":
		return "System"
	default:
		return namespace
	}
}

func canonicalApexMemberName(memberName string) string {
	memberName = cleanIdentityPart(memberName)
	switch memberName {
	case "setFilterID":
		return "setFilterId"
	case "setpageNumber":
		return "setPageNumber"
	case "getSOAPType":
		return "getSoapType"
	case "getSObjectField":
		return "getSobjectField"
	default:
		return memberName
	}
}

func cleanIdentityPart(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}
