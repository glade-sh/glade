package capability

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ToolingCompletions is the public shape returned by Salesforce Tooling API
// /services/data/<version>/tooling/completions/?type=apex.
type ToolingCompletions struct {
	PublicDeclarations map[string]map[string]ToolingClassDecl `json:"publicDeclarations"`
}

type ToolingClassDecl struct {
	Constructors []ToolingConstructor `json:"constructors"`
	Methods      []ToolingMethod      `json:"methods"`
	Properties   []ToolingProperty    `json:"properties"`
}

type ToolingConstructor struct {
	Name       string             `json:"name"`
	Parameters []ToolingParameter `json:"parameters"`
}

type ToolingMethod struct {
	Name       string             `json:"name"`
	ReturnType string             `json:"returnType"`
	IsStatic   bool               `json:"isStatic"`
	Parameters []ToolingParameter `json:"parameters"`
	ArgTypes   []string           `json:"argTypes"`
}

type ToolingProperty struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ToolingParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ToolingApexClassSymbols struct {
	Records []ToolingApexClassRecord `json:"records"`
}

type ToolingApexClassRecord struct {
	ID              string              `json:"Id"`
	Name            string              `json:"Name"`
	NamespacePrefix string              `json:"NamespacePrefix"`
	SymbolTable     *ToolingSymbolTable `json:"SymbolTable"`
}

type ToolingSymbolTable struct {
	Constructors       []ToolingSymbolMethod      `json:"constructors"`
	Methods            []ToolingSymbolMethod      `json:"methods"`
	Properties         []ToolingSymbolProperty    `json:"properties"`
	InnerClasses       []ToolingSymbolTable       `json:"innerClasses"`
	ParentClass        string                     `json:"parentClass"`
	Interfaces         []string                   `json:"interfaces"`
	Namespace          *string                    `json:"namespace"`
	Name               string                     `json:"name"`
	TableDeclaration   ToolingSymbolDecl          `json:"tableDeclaration"`
	Variables          []ToolingSymbolVariable    `json:"variables"`
	ExternalReferences []ToolingExternalReference `json:"externalReferences"`
}

type ToolingSymbolMethod struct {
	Name       string             `json:"name"`
	ReturnType string             `json:"returnType"`
	Parameters []ToolingParameter `json:"parameters"`
	Modifiers  []string           `json:"modifiers"`
	Visibility string             `json:"visibility"`
}

type ToolingSymbolProperty struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Modifiers  []string `json:"modifiers"`
	Visibility string   `json:"visibility"`
}

type ToolingSymbolVariable struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Modifiers []string `json:"modifiers"`
}

type ToolingSymbolDecl struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Modifiers []string `json:"modifiers"`
}

type ToolingExternalReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func ReadToolingCompletions(path string) (ToolingCompletions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolingCompletions{}, err
	}
	var completions ToolingCompletions
	if err := json.Unmarshal(data, &completions); err != nil {
		return ToolingCompletions{}, fmt.Errorf("read Tooling API completions %s: %w", path, err)
	}
	NormalizeToolingCompletions(&completions)
	return completions, nil
}

func ReadToolingApexClassSymbols(path string) (ToolingApexClassSymbols, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolingApexClassSymbols{}, err
	}
	var symbols ToolingApexClassSymbols
	if err := json.Unmarshal(data, &symbols); err != nil {
		var records []ToolingApexClassRecord
		if arrayErr := json.Unmarshal(data, &records); arrayErr != nil {
			return ToolingApexClassSymbols{}, fmt.Errorf("read Tooling API ApexClass symbols %s: %w", path, err)
		}
		symbols.Records = records
	}
	return symbols, nil
}

func NormalizeToolingCompletions(completions *ToolingCompletions) {
	if completions == nil || completions.PublicDeclarations == nil {
		return
	}
	normalizeToolingClassNames(completions)
	normalizeToolingExceptionConstructors(completions)
	normalizeToolingDatabaseTypes(completions)
	normalizeToolingSchemaTypes(completions)
	normalizeToolingConnectApi(completions)
}

func normalizeToolingClassNames(completions *ToolingCompletions) {
	system, ok := completions.PublicDeclarations["System"]
	if !ok {
		return
	}
	for from, to := range map[string]string{"LIST": "List", "MAP": "Map", "SET": "Set"} {
		if decl, ok := system[from]; ok {
			system[to] = decl
			delete(system, from)
		}
	}
}

func normalizeToolingExceptionConstructors(completions *ToolingCompletions) {
	system, ok := completions.PublicDeclarations["System"]
	if !ok {
		return
	}
	standard := []ToolingConstructor{
		{Name: "Exception"},
		{Name: "Exception", Parameters: []ToolingParameter{{Name: "param1", Type: "String"}}},
		{Name: "Exception", Parameters: []ToolingParameter{{Name: "param1", Type: "String"}, {Name: "param2", Type: "Exception"}}},
		{Name: "Exception", Parameters: []ToolingParameter{{Name: "param1", Type: "Exception"}}},
	}
	for className, decl := range system {
		if !strings.HasSuffix(className, "Exception") || len(decl.Constructors) >= len(standard) {
			continue
		}
		seen := map[int]bool{}
		for _, ctor := range decl.Constructors {
			seen[len(ctor.Parameters)] = true
		}
		if len(decl.Constructors) == 0 {
			decl.Constructors = append([]ToolingConstructor(nil), standard...)
		} else {
			for _, ctor := range standard {
				if !seen[len(ctor.Parameters)] {
					decl.Constructors = append(decl.Constructors, ctor)
				}
			}
		}
		system[className] = decl
	}
}

func normalizeToolingDatabaseTypes(completions *ToolingCompletions) {
	system, ok := completions.PublicDeclarations["System"]
	if !ok {
		return
	}
	database, ok := system["Database"]
	if !ok {
		return
	}
	for i := range database.Methods {
		for j := range database.Methods[i].Parameters {
			param := &database.Methods[i].Parameters[j]
			if param.Type != "APEX_OBJECT" {
				continue
			}
			name := strings.ToLower(param.Name)
			switch {
			case strings.Contains(name, "accesslevel") || strings.Contains(name, "access_level"):
				param.Type = "System.AccessLevel"
			case strings.Contains(name, "dmloptions") || strings.Contains(name, "dml_options"):
				param.Type = "Database.DMLOptions"
			case strings.Contains(name, "callback"):
				param.Type = "Database.AllowCallouts"
			}
		}
	}
	system["Database"] = database
}

func normalizeToolingSchemaTypes(completions *ToolingCompletions) {
	schema, ok := completions.PublicDeclarations["Schema"]
	if !ok {
		return
	}
	if sobjectType, ok := schema["SObjectType"]; ok {
		for i := range sobjectType.Methods {
			method := &sobjectType.Methods[i]
			if method.Name == "getDescribe" && len(method.Parameters) == 0 {
				method.ReturnType = "Schema.DescribeSObjectResult"
			}
			if method.Name == "newSObject" {
				method.ReturnType = "SObject"
			}
		}
		schema["SObjectType"] = sobjectType
	}
	if sobjectField, ok := schema["SObjectField"]; ok {
		for i := range sobjectField.Methods {
			method := &sobjectField.Methods[i]
			if method.Name == "getDescribe" && len(method.Parameters) == 0 {
				method.ReturnType = "Schema.DescribeFieldResult"
			}
		}
		schema["SObjectField"] = sobjectField
	}
}

func normalizeToolingConnectApi(completions *ToolingCompletions) {
	connectApi, ok := completions.PublicDeclarations["ConnectApi"]
	if !ok {
		return
	}
	if _, ok := connectApi["ChatterFeeds"]; !ok {
		connectApi["ChatterFeeds"] = ToolingClassDecl{Methods: []ToolingMethod{{
			Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", IsStatic: true,
			Parameters: []ToolingParameter{{Name: "communityId", Type: "String"}, {Name: "feedElement", Type: "ConnectApi.FeedElementInput"}},
		}, {
			Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", IsStatic: true,
			Parameters: []ToolingParameter{{Name: "communityId", Type: "String"}, {Name: "subjectId", Type: "String"}, {Name: "feedElementType", Type: "ConnectApi.FeedElementType"}, {Name: "text", Type: "String"}},
		}}}
	}
	if _, ok := connectApi["UserProfiles"]; !ok {
		connectApi["UserProfiles"] = ToolingClassDecl{Methods: []ToolingMethod{{
			Name: "getUserProfile", ReturnType: "ConnectApi.UserProfile", IsStatic: true,
			Parameters: []ToolingParameter{{Name: "communityId", Type: "String"}, {Name: "userId", Type: "String"}},
		}}}
	}
}
