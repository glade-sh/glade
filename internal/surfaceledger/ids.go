package surfaceledger

import (
	"strings"
)

func ApexTypeID(namespace, typeName string) string {
	return "apex:" + qualifiedName(namespace, typeName)
}

func ApexMemberID(namespace, typeName, memberName string, parameters []string) string {
	id := ApexTypeID(namespace, typeName) + "." + strings.TrimSpace(memberName)
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
	namespace = strings.TrimSpace(namespace)
	typeName = strings.TrimSpace(typeName)
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
			out = append(out, value)
		}
	}
	return out
}
