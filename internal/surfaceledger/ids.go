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

func DataObjectID(objectName string) string {
	return "data-reference:" + strings.TrimSpace(objectName)
}

func DataFieldID(objectName, fieldName string) string {
	return DataObjectID(objectName) + "." + strings.TrimSpace(fieldName)
}

func RestResourceID(resource, method string) string {
	return "rest:" + strings.Trim(strings.TrimSpace(resource), "/") + "." + asciiLowerIdentityKey(strings.TrimSpace(method))
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
	switch {
	case strings.EqualFold(value, "APEX_OBJECT"):
		return "Object"
	case strings.EqualFold(value, "ANY"):
		return "Object"
	case strings.EqualFold(value, "SOBJECT"):
		return "Object"
	case strings.EqualFold(value, "SOBJECT[]"):
		return "List<Object>"
	case strings.EqualFold(value, "SYSTEM.TYPE"):
		return "Type"
	case strings.EqualFold(value, "BATCHABLE"), strings.EqualFold(value, "DATABASE.BATCHABLE"), strings.EqualFold(value, "SYSTEM.DATABASE.BATCHABLE"):
		return "Object"
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
	if (namespace == "System" || namespace == "Schema") && typeName == "Schema" {
		namespace = "Schema"
	}
	if namespace == "System" {
		switch typeName {
		case "ChildRelationship", "DataCategory", "DataCategoryGroupSobjectTypePair", "DescribeColorResult", "DescribeDataCategoryGroupResult", "DescribeDataCategoryGroupStructureResult", "DescribeFieldResult", "DescribeIconResult", "DescribeSObjectResult", "DescribeTabResult", "DescribeTabSetResult", "FieldSet", "FieldSetMap", "FieldSetMember", "PicklistEntry", "RecordTypeInfo", "SObjectField", "SObjectType":
			namespace = "Schema"
		case "QueryLocator", "QueryLocatorChunkIterator", "QueryLocatorIterator":
			namespace = "Database"
		}
	}
	return namespace, typeName
}

func canonicalApexNamespaceName(namespace string) string {
	namespace = cleanIdentityPart(namespace)
	if known, ok := canonicalKnownApexName(namespace, canonicalApexNamespaces); ok {
		return known
	}
	return namespace
}

func canonicalApexMemberName(memberName string) string {
	return cleanIdentityPart(memberName)
}

func surfaceIDKey(id string) string {
	id = cleanIdentityPart(id)
	if strings.HasPrefix(id, "apex:") {
		rest := strings.TrimPrefix(id, "apex:")
		if strings.HasPrefix(rest, "System.QueryLocator") {
			rest = "Database.QueryLocator" + strings.TrimPrefix(rest, "System.QueryLocator")
		}
		rest = strings.ReplaceAll(rest, "(List,System.AccessLevel)", "(List<Object>,System.AccessLevel)")
		folded := asciiLowerIdentityKey(rest)
		if folded == rest {
			return "apex:" + rest
		}
		return "apex:" + folded
	}
	if strings.HasPrefix(id, "data-reference:") {
		rest := strings.TrimPrefix(id, "data-reference:")
		folded := asciiLowerIdentityKey(rest)
		if folded == rest {
			return id
		}
		return "data-reference:" + folded
	}
	return id
}

func asciiLowerIdentityKey(value string) string {
	for i := 0; i < len(value); i++ {
		if value[i] >= 'A' && value[i] <= 'Z' {
			buf := []byte(value)
			for j := i; j < len(buf); j++ {
				if buf[j] >= 'A' && buf[j] <= 'Z' {
					buf[j] += 'a' - 'A'
				}
			}
			return string(buf)
		}
	}
	return value
}

var canonicalApexNamespaces = []string{
	"Cache",
	"ConnectApi",
	"Database",
	"Schema",
	"System",
}

func canonicalKnownApexName(name string, known []string) (string, bool) {
	for _, candidate := range known {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func containsASCIIFold(value, substr string) bool {
	if substr == "" {
		return true
	}
	if len(substr) > len(value) {
		return false
	}
	for i := 0; i <= len(value)-len(substr); i++ {
		if asciiEqualFoldAt(value, substr, i) {
			return true
		}
	}
	return false
}

func asciiEqualFoldAt(value, substr string, offset int) bool {
	for i := 0; i < len(substr); i++ {
		left := value[offset+i]
		right := substr[i]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}

func cleanIdentityPart(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value)
	value = strings.ReplaceAll(value, `\_`, "_")
	return strings.TrimSpace(value)
}
