package apexlang

import "strings"

type AnnotationSpec struct {
	Name             string
	Preview          bool
	AllowedArguments map[string]struct{}
}

var annotations = []AnnotationSpec{
	{Name: "AuraEnabled", AllowedArguments: set("cacheable")},
	{Name: "future", AllowedArguments: set("callout")},
	{Name: "InvocableMethod", AllowedArguments: set("label", "description", "category", "callout")},
	{Name: "InvocableVariable", AllowedArguments: set("label", "description", "required")},
	{Name: "IsTest", AllowedArguments: set("SeeAllData", "IsParallel", "OnInstall")},
	{Name: "TestSetup"},
	{Name: "TestVisible"},
	{Name: "SuppressWarnings"},
	{Name: "Deprecated"},
	{Name: "JsonAccess", AllowedArguments: set("serializable", "deserializable")},
	{Name: "NamespaceAccessible"},
	{Name: "ReadOnly"},
	{Name: "RemoteAction"},
	{Name: "RestResource", AllowedArguments: set("urlMapping")},
	{Name: "HttpGet"}, {Name: "HttpPost"}, {Name: "HttpPut"}, {Name: "HttpPatch"}, {Name: "HttpDelete"},
	{Name: "webservice"},
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

func set(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[strings.ToLower(value)] = struct{}{}
	}
	return out
}
