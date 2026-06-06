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
	id := ApexTypeID(namespace, typeName) + "." + canonicalApexMemberNameForType(typeName, memberName)
	if parameters != nil {
		id += "(" + strings.Join(canonicalApexMemberParameters(namespace, typeName, memberName, parameters), ",") + ")"
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
	value = strings.ReplaceAll(value, ", ", ",")
	if strings.HasSuffix(value, "[]") {
		return "List<" + canonicalParameterType(strings.TrimSuffix(value, "[]")) + ">"
	}
	switch {
	case strings.EqualFold(value, "ID"):
		return "Id"
	case strings.EqualFold(value, "APEX_OBJECT"):
		return "Object"
	case strings.EqualFold(value, "ANY"):
		return "Object"
	case strings.EqualFold(value, "SOBJECT"):
		return "Object"
	case strings.EqualFold(value, "SOBJECT[]"):
		return "List<Object>"
	case strings.EqualFold(value, "MAPTOCOPY"):
		return "Map"
	case strings.EqualFold(value, "T"), strings.EqualFold(value, "T1"), strings.EqualFold(value, "T2"), strings.EqualFold(value, "KEY"):
		return "Object"
	case strings.EqualFold(value, "LIST<T>"):
		return "List"
	case strings.EqualFold(value, "MAP<T1,T2>"):
		return "Map"
	case strings.EqualFold(value, "SET<T>"):
		return "Set"
	case strings.EqualFold(value, "SYSTEM.TYPE"):
		return "Type"
	case strings.EqualFold(value, "BATCHABLE"), strings.EqualFold(value, "DATABASE.BATCHABLE"), strings.EqualFold(value, "SYSTEM.DATABASE.BATCHABLE"):
		return "Object"
	default:
		value = stripSystemTypeQualifier(value)
		value = strings.ReplaceAll(value, "<ANY>", "<Object>")
		value = strings.ReplaceAll(value, "<APEX_OBJECT>", "<Object>")
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
		case "QueryLocator", "QueryLocatorChunkIterator", "QueryLocatorIterator", "DeleteResult", "DMLOptions", "EmptyRecycleBinResult", "Error", "SaveResult", "UndeleteResult", "UpsertResult":
			namespace = "Database"
		case "Answers", "Approval", "BusinessHours", "Ideas", "QueueableDuplicateSignature", "QueueableDuplicateSignature.Builder":
			namespace = ""
		}
	}
	return namespace, typeName
}

func canonicalApexMemberParameters(namespace, typeName, memberName string, parameters []string) []string {
	out := cleanList(parameters)
	if namespace == "" && (typeName == "Answers" || typeName == "Ideas") && memberName == "findSimilar" {
		if len(out) > 0 && (out[0] == "Question" || out[0] == "Idea") {
			out[0] = "Object"
		}
	}
	if namespace == "System" && typeName == "System" {
		switch memberName {
		case "getQuiddityShortCode":
			if len(out) == 1 && out[0] == "Quiddity" {
				out[0] = "Object"
			}
		case "process":
			if len(out) == 4 && strings.EqualFold(out[0], "List<Id>") {
				out[0] = "List"
			}
		case "submit":
			if len(out) == 3 && strings.EqualFold(out[0], "List<Id>") {
				out[0] = "List"
			}
		case "runAs":
			if len(out) == 1 && out[0] == "Version" {
				out[0] = "Package.Version"
			}
		}
	}
	return out
}

func canonicalApexNamespaceName(namespace string) string {
	namespace = cleanIdentityPart(namespace)
	if known, ok := canonicalKnownApexName(namespace, canonicalApexNamespaces); ok {
		return known
	}
	return namespace
}

func canonicalApexMemberName(memberName string) string {
	memberName = cleanIdentityPart(memberName)
	if memberName == "publishWithAcessLevel" {
		return "publishWithAccessLevel"
	}
	return memberName
}

func surfaceIDKey(id string) string {
	id = cleanIdentityPart(id)
	if strings.HasPrefix(id, "apex:") {
		rest := strings.TrimPrefix(id, "apex:")
		if strings.HasPrefix(rest, "System.QueryLocator") {
			rest = "Database.QueryLocator" + strings.TrimPrefix(rest, "System.QueryLocator")
		}
		rest = canonicalApexIDConstructorName(rest)
		rest = canonicalApexIDParameterList(rest)
		rest = strings.ReplaceAll(rest, "System.Comparator.compare(T,T)", "System.Comparator.compare(Object,Object)")
		rest = strings.ReplaceAll(rest, "(List,System.AccessLevel)", "(List<Object>,System.AccessLevel)")
		rest = strings.ReplaceAll(rest, "(List,AccessLevel)", "(List<Object>,AccessLevel)")
		rest = strings.ReplaceAll(rest, "System.Database.getQueryLocator(String,Object)", "System.Database.getQueryLocator(String,AccessLevel)")
		rest = strings.ReplaceAll(rest, "System.Database.getQueryLocatorWithBinds(String,Map,Object)", "System.Database.getQueryLocatorWithBinds(String,Map,AccessLevel)")
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

func canonicalApexIDParameterList(rest string) string {
	open := strings.IndexByte(rest, '(')
	if open < 0 || !strings.HasSuffix(rest, ")") {
		return rest
	}
	params := rest[open+1 : len(rest)-1]
	return rest[:open+1] + strings.Join(cleanList(splitSurfaceParameterList(params)), ",") + ")"
}

func canonicalApexIDConstructorName(rest string) string {
	open := strings.IndexByte(rest, '(')
	beforeParams := rest
	afterParams := ""
	if open >= 0 {
		beforeParams = rest[:open]
		afterParams = rest[open:]
	}
	memberDot := strings.LastIndexByte(beforeParams, '.')
	if memberDot <= 0 || memberDot == len(beforeParams)-1 {
		return rest
	}
	typePart := beforeParams[:memberDot]
	typeDot := strings.LastIndexByte(typePart, '.')
	typeName := typePart
	if typeDot >= 0 {
		typeName = typePart[typeDot+1:]
	}
	memberName := beforeParams[memberDot+1:]
	if strings.HasPrefix(memberName, typeName+"<") {
		return typePart + "." + typeName + afterParams
	}
	return rest
}

func canonicalApexMemberNameForType(typeName, memberName string) string {
	memberName = canonicalApexMemberName(memberName)
	if strings.HasPrefix(memberName, typeName+"<") {
		return typeName
	}
	return memberName
}

func stripSystemTypeQualifier(value string) string {
	if !strings.Contains(value, "System.") {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); {
		if strings.HasPrefix(value[i:], "System.") && (i == 0 || isTypeQualifierBoundary(value[i-1])) {
			i += len("System.")
			continue
		}
		out.WriteByte(value[i])
		i++
	}
	return out.String()
}

func isTypeQualifierBoundary(ch byte) bool {
	switch ch {
	case '<', ',', ' ', '(':
		return true
	default:
		return false
	}
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
