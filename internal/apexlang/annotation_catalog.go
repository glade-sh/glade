package apexlang

import "strings"

type AnnotationArgumentKind string

const (
	AnnotationStringArgument  AnnotationArgumentKind = "string"
	AnnotationBooleanArgument AnnotationArgumentKind = "boolean"
)

type AnnotationSpec struct {
	Name                      string
	Preview                   bool
	MaxPositionalArguments    int
	PositionalArgumentLiteral bool
	Arguments                 map[string]AnnotationArgumentKind
}

var annotations = []AnnotationSpec{
	{Name: "AuraEnabled", Arguments: arguments(stringArguments("scope"), booleanArguments("cacheable"))},
	{Name: "future", Arguments: arguments(booleanArguments("callout"))},
	{Name: "InvocableMethod", Arguments: arguments(stringArguments("label", "description", "capabilityType", "category", "configurationEditor", "iconName"), booleanArguments("callout"))},
	{Name: "InvocableVariable", Arguments: arguments(stringArguments("label", "description", "defaultValue", "placeholderText"), booleanArguments("required"))},
	{Name: "IsTest", Arguments: arguments(stringArguments("testFor"), booleanArguments("SeeAllData", "IsParallel", "OnInstall", "critical"))},
	{Name: "TestSetup"},
	{Name: "TestVisible"},
	{Name: "SuppressWarnings", MaxPositionalArguments: 1, PositionalArgumentLiteral: true},
	{Name: "Deprecated"},
	{Name: "JsonAccess", Arguments: arguments(booleanArguments("serializable", "deserializable"))},
	{Name: "NamespaceAccessible"},
	{Name: "ReadOnly"},
	{Name: "RemoteAction"},
	{Name: "RestResource", Arguments: arguments(stringArguments("urlMapping"))},
	{Name: "HttpGet"}, {Name: "HttpPost"}, {Name: "HttpPut"}, {Name: "HttpPatch"}, {Name: "HttpDelete"},
	{Name: "IntegrationTest", Preview: true},
	{Name: "TearDown", Preview: true},
}

func AllAnnotations() []AnnotationSpec {
	out := make([]AnnotationSpec, len(annotations))
	copy(out, annotations)
	return out
}

func LookupAnnotation(name string) (AnnotationSpec, bool) {
	for _, item := range annotations {
		if strings.EqualFold(item.Name, strings.TrimPrefix(strings.TrimSpace(name), "@")) {
			return item, true
		}
	}
	return AnnotationSpec{}, false
}

func arguments(groups ...map[string]AnnotationArgumentKind) map[string]AnnotationArgumentKind {
	out := make(map[string]AnnotationArgumentKind)
	for _, group := range groups {
		for name, kind := range group {
			out[name] = kind
		}
	}
	return out
}

func stringArguments(values ...string) map[string]AnnotationArgumentKind {
	return annotationArguments(AnnotationStringArgument, values...)
}

func booleanArguments(values ...string) map[string]AnnotationArgumentKind {
	return annotationArguments(AnnotationBooleanArgument, values...)
}

func annotationArguments(kind AnnotationArgumentKind, values ...string) map[string]AnnotationArgumentKind {
	out := make(map[string]AnnotationArgumentKind, len(values))
	for _, value := range values {
		out[strings.ToLower(value)] = kind
	}
	return out
}
