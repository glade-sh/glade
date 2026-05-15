package typesys

import "github.com/open-aer/oaer/internal/apexast"

// Code generated from public Salesforce product namespace declarations. DO NOT EDIT.

var productNamespaceSymbolSpecs = []StandardSymbolSpec{
	{
		Name: "Cache.Org",
		Methods: []StandardMethodSpec{
			{Name: "contains", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true},
			{Name: "get", ReturnType: "Object", Parameters: []string{"String"}, Static: true},
			{Name: "get", ReturnType: "Object", Parameters: []string{"Type", "String"}, Static: true},
			{Name: "getCapacity", ReturnType: "Double", Static: true},
			{Name: "getKeys", ReturnType: "Set<String>", Static: true},
			{Name: "getName", ReturnType: "String", Static: true},
			{Name: "getNumKeys", ReturnType: "Long", Static: true},
			{Name: "getPartition", ReturnType: "Cache.OrgPartition", Parameters: []string{"String"}, Static: true},
			{Name: "isAvailable", ReturnType: "Boolean", Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object"}, Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer"}, Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer", "Cache.Visibility", "Boolean"}, Static: true},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"String"}, Static: true},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"Type", "String"}, Static: true},
		},
	},
	{
		Name: "Cache.OrgPartition",
		Methods: []StandardMethodSpec{
			{Name: "contains", ReturnType: "Boolean", Parameters: []string{"String"}},
			{Name: "get", ReturnType: "Object", Parameters: []string{"String"}},
			{Name: "get", ReturnType: "Object", Parameters: []string{"Type", "String"}},
			{Name: "getKeys", ReturnType: "Set<String>"},
			{Name: "getNumKeys", ReturnType: "Long"},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object"}},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer"}},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer", "Cache.Visibility", "Boolean"}},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"String"}},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"Type", "String"}},
		},
	},
	{
		Name: "Cache.Session",
		Methods: []StandardMethodSpec{
			{Name: "contains", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true},
			{Name: "get", ReturnType: "Object", Parameters: []string{"String"}, Static: true},
			{Name: "get", ReturnType: "Object", Parameters: []string{"Type", "String"}, Static: true},
			{Name: "getCapacity", ReturnType: "Double", Static: true},
			{Name: "getKeys", ReturnType: "Set<String>", Static: true},
			{Name: "getName", ReturnType: "String", Static: true},
			{Name: "getNumKeys", ReturnType: "Long", Static: true},
			{Name: "getPartition", ReturnType: "Cache.SessionPartition", Parameters: []string{"String"}, Static: true},
			{Name: "isAvailable", ReturnType: "Boolean", Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object"}, Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer"}, Static: true},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer", "Cache.Visibility", "Boolean"}, Static: true},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"String"}, Static: true},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"Type", "String"}, Static: true},
		},
	},
	{
		Name: "Cache.SessionPartition",
		Methods: []StandardMethodSpec{
			{Name: "contains", ReturnType: "Boolean", Parameters: []string{"String"}},
			{Name: "get", ReturnType: "Object", Parameters: []string{"String"}},
			{Name: "get", ReturnType: "Object", Parameters: []string{"Type", "String"}},
			{Name: "getKeys", ReturnType: "Set<String>"},
			{Name: "getNumKeys", ReturnType: "Long"},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object"}},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer"}},
			{Name: "put", ReturnType: "void", Parameters: []string{"String", "Object", "Integer", "Cache.Visibility", "Boolean"}},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"String"}},
			{Name: "remove", ReturnType: "Object", Parameters: []string{"Type", "String"}},
		},
	},
	{
		Name: "Cache.Visibility",
		Kind: apexast.DeclarationEnum,
		Properties: []StandardPropertySpec{
			{Name: "ALL", Type: "Cache.Visibility", Static: true},
			{Name: "NAMESPACE", Type: "Cache.Visibility", Static: true},
		},
	},
	{
		Name: "ConnectApi.Communities",
		Methods: []StandardMethodSpec{
			{Name: "getCommunity", ReturnType: "ConnectApi.Community", Parameters: []string{"String"}, Static: true},
		},
	},
	{
		Name: "ConnectApi.Community",
		Properties: []StandardPropertySpec{
			{Name: "id", Type: "String"},
			{Name: "name", Type: "String"},
			{Name: "siteUrl", Type: "String"},
		},
	},
	{
		Name: "ConnectApi.Organization",
		Methods: []StandardMethodSpec{
			{Name: "getSettings", ReturnType: "ConnectApi.OrganizationSettings", Static: true},
		},
	},
	{
		Name: "ConnectApi.OrganizationSettings",
		Properties: []StandardPropertySpec{
			{Name: "defaultLanguage", Type: "String"},
			{Name: "defaultLocale", Type: "String"},
			{Name: "defaultTimeZone", Type: "ConnectApi.TimeZone"},
			{Name: "id", Type: "String"},
			{Name: "name", Type: "String"},
			{Name: "userSettings", Type: "ConnectApi.UserSettings"},
		},
	},
	{
		Name: "ConnectApi.TimeZone",
		Properties: []StandardPropertySpec{
			{Name: "displayName", Type: "String"},
			{Name: "id", Type: "String"},
			{Name: "offset", Type: "Integer"},
		},
	},
	{
		Name: "ConnectApi.UserProfiles",
		Methods: []StandardMethodSpec{
			{Name: "deletePhoto", ReturnType: "void", Parameters: []string{"String", "String"}, Static: true},
			{Name: "getUserProfile", ReturnType: "ConnectApi.UserProfile", Parameters: []string{"String", "String"}, Static: true},
			{Name: "setPhoto", ReturnType: "void", Parameters: []string{"String", "String", "String", "Object"}, Static: true},
		},
	},
	{
		Name: "ConnectApi.UserSettings",
		Properties: []StandardPropertySpec{
			{Name: "timeZone", Type: "ConnectApi.TimeZone"},
		},
	},
	{
		Name:         "Metadata.AsyncResult",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "done", Type: "Boolean"},
			{Name: "id", Type: "Id"},
			{Name: "message", Type: "String"},
			{Name: "state", Type: "String"},
			{Name: "statusCode", Type: "Metadata.StatusCode"},
		},
	},
	{
		Name:         "Metadata.CustomField",
		SuperClass:   "Metadata.Metadata",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "description", Type: "String"},
			{Name: "fullName", Type: "String"},
			{Name: "label", Type: "String"},
			{Name: "required", Type: "Boolean"},
			{Name: "type", Type: "String"},
			{Name: "unique", Type: "Boolean"},
		},
	},
	{
		Name:         "Metadata.CustomMetadata",
		SuperClass:   "Metadata.Metadata",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "description", Type: "String"},
			{Name: "fullName", Type: "String"},
			{Name: "label", Type: "String"},
			{Name: "protected_x", Type: "Boolean"},
			{Name: "values", Type: "List<Metadata.CustomMetadataValue>"},
		},
	},
	{
		Name:         "Metadata.CustomMetadataValue",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "field", Type: "String"},
			{Name: "value", Type: "Object"},
		},
	},
	{
		Name:         "Metadata.CustomObject",
		SuperClass:   "Metadata.Metadata",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "deploymentStatus", Type: "String"},
			{Name: "description", Type: "String"},
			{Name: "enableActivities", Type: "Boolean"},
			{Name: "enableReports", Type: "Boolean"},
			{Name: "fullName", Type: "String"},
			{Name: "label", Type: "String"},
			{Name: "pluralLabel", Type: "String"},
		},
	},
	{
		Name: "Metadata.DeployCallback",
		Kind: apexast.DeclarationInterface,
		Methods: []StandardMethodSpec{
			{Name: "handleResult", ReturnType: "void", Parameters: []string{"Metadata.DeployResult", "Metadata.DeployCallbackContext"}},
		},
	},
	{
		Name:         "Metadata.DeployCallbackContext",
		Constructors: [][]string{{}},
		Methods: []StandardMethodSpec{
			{Name: "getCallbackJobId", ReturnType: "Id"},
		},
	},
	{
		Name:         "Metadata.DeployContainer",
		Constructors: [][]string{{}},
		Methods: []StandardMethodSpec{
			{Name: "addMetadata", ReturnType: "void", Parameters: []string{"Metadata.Metadata"}},
			{Name: "getMetadata", ReturnType: "List<Metadata.Metadata>"},
			{Name: "removeMetadata", ReturnType: "void", Parameters: []string{"Metadata.Metadata"}},
			{Name: "removeMetadataByFullName", ReturnType: "void", Parameters: []string{"String"}},
		},
	},
	{
		Name:         "Metadata.DeployDetails",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "componentFailures", Type: "List<Metadata.DeployMessage>"},
			{Name: "componentSuccesses", Type: "List<Metadata.DeployMessage>"},
		},
	},
	{
		Name:         "Metadata.DeployMessage",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "changed", Type: "Boolean"},
			{Name: "componentType", Type: "String"},
			{Name: "fileName", Type: "String"},
			{Name: "fullName", Type: "String"},
			{Name: "problem", Type: "String"},
			{Name: "problemType", Type: "String"},
			{Name: "success", Type: "Boolean"},
		},
	},
	{
		Name:         "Metadata.DeployResult",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "details", Type: "Metadata.DeployDetails"},
			{Name: "done", Type: "Boolean"},
			{Name: "id", Type: "Id"},
			{Name: "status", Type: "Metadata.DeployStatus"},
			{Name: "success", Type: "Boolean"},
		},
	},
	{
		Name: "Metadata.DeployStatus",
		Kind: apexast.DeclarationEnum,
		Properties: []StandardPropertySpec{
			{Name: "CANCELED", Type: "Metadata.DeployStatus", Static: true},
			{Name: "FAILED", Type: "Metadata.DeployStatus", Static: true},
			{Name: "IN_PROGRESS", Type: "Metadata.DeployStatus", Static: true},
			{Name: "PENDING", Type: "Metadata.DeployStatus", Static: true},
			{Name: "SUCCEEDED", Type: "Metadata.DeployStatus", Static: true},
		},
	},
	{
		Name:         "Metadata.Metadata",
		Constructors: [][]string{{}},
		Properties: []StandardPropertySpec{
			{Name: "fullName", Type: "String"},
		},
	},
	{
		Name: "Metadata.MetadataType",
		Kind: apexast.DeclarationEnum,
		Properties: []StandardPropertySpec{
			{Name: "CustomMetadata", Type: "Metadata.MetadataType", Static: true},
		},
	},
	{
		Name: "Metadata.Operations",
		Methods: []StandardMethodSpec{
			{Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"Id"}, Static: true},
			{Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"Id", "Boolean"}, Static: true},
			{Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"String"}, Static: true},
			{Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"String", "Boolean"}, Static: true},
			{Name: "enqueueDeployment", ReturnType: "Id", Parameters: []string{"Metadata.DeployContainer", "Metadata.DeployCallback"}, Static: true},
			{Name: "retrieve", ReturnType: "List<Metadata.CustomMetadata>", Parameters: []string{"Metadata.MetadataType", "List<String>"}, Static: true},
			{Name: "retrieve", ReturnType: "List<Metadata.CustomMetadata>", Parameters: []string{"Metadata.MetadataType", "List<String>", "Boolean"}, Static: true},
		},
	},
	{
		Name: "Metadata.StatusCode",
		Kind: apexast.DeclarationEnum,
	},
}
