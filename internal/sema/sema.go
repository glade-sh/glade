package sema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/typesys"
)

type Result struct {
	Project     typesys.ProjectInfo      `json:"project"`
	Summary     Summary                  `json:"summary"`
	Diagnostics []diagnostic.Diagnostic  `json:"diagnostics,omitempty"`
	Types       map[string]TypeReference `json:"types,omitempty"`
}

type Summary struct {
	Types       int `json:"types"`
	Triggers    int `json:"triggers"`
	Objects     int `json:"objects"`
	Diagnostics int `json:"diagnostics"`
}

type TypeReference struct {
	Name   string   `json:"name"`
	Kind   TypeKind `json:"kind"`
	Source string   `json:"source,omitempty"`
}

type TypeKind string

const (
	TypeApex     TypeKind = "apex"
	TypeSchema   TypeKind = "schema"
	TypePlatform TypeKind = "platform"
	TypeBuiltin  TypeKind = "builtin"
)

type Analyzer struct {
	known map[string]TypeReference
}

func NewAnalyzer() *Analyzer {
	a := &Analyzer{known: make(map[string]TypeReference)}
	for _, name := range builtinTypes {
		a.addKnown(name, TypeBuiltin, "")
	}
	for _, name := range platformTypes {
		a.addKnown(name, TypePlatform, "")
	}
	return a
}

func Analyze(index typesys.Index) Result {
	return NewAnalyzer().Analyze(index)
}

func (a *Analyzer) Analyze(index typesys.Index) Result {
	result := Result{
		Project:     index.Project,
		Diagnostics: append([]diagnostic.Diagnostic{}, index.Diagnostics...),
	}

	for _, object := range index.Objects {
		a.addKnown(object.Name, TypeSchema, "")
	}
	for _, typ := range index.Types {
		a.addKnown(typ.Name, TypeApex, typ.File)
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationClass || member.Kind == apexast.DeclarationInterface || member.Kind == apexast.DeclarationEnum {
				a.addKnown(member.Name, TypeApex, typ.File)
				a.addKnown(typ.Name+"."+member.Name, TypeApex, typ.File)
			}
		}
	}

	result.Diagnostics = append(result.Diagnostics, a.checkTriggers(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMemberTypes(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkSchemaReferences(index)...)

	result.Summary = Summary{
		Types:       len(index.Types),
		Triggers:    len(index.Triggers),
		Objects:     len(index.Objects),
		Diagnostics: len(result.Diagnostics),
	}
	result.Types = a.exportKnownTypes()
	return result
}

func (r Result) HasErrors() bool {
	for _, diag := range r.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkTriggers(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, trigger := range index.Triggers {
		if trigger.ObjectName == "" || a.hasKnown(trigger.ObjectName) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA001",
			Message:  fmt.Sprintf("trigger %q references unknown SObject %q", trigger.Name, trigger.ObjectName),
			File:     trigger.File,
			Range:    &trigger.Range,
		})
	}
	return diagnostics
}

func (a *Analyzer) checkMemberTypes(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		for _, member := range typ.Members {
			if member.Type == "" || member.Type == "void" {
				continue
			}
			for _, ref := range extractTypeNames(member.Type) {
				if a.hasKnown(ref) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA002",
					Message:  fmt.Sprintf("%s %q references unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkSchemaReferences(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, object := range index.Objects {
		for _, field := range object.Fields {
			if field.ReferenceTo == "" || a.hasKnown(field.ReferenceTo) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA003",
				Message:  fmt.Sprintf("field %s.%s references unknown SObject %q", object.Name, field.Name, field.ReferenceTo),
			})
		}
	}
	return diagnostics
}

func (a *Analyzer) addKnown(name string, kind TypeKind, source string) {
	if name == "" {
		return
	}
	key := normalizeName(name)
	if _, exists := a.known[key]; exists {
		return
	}
	a.known[key] = TypeReference{Name: name, Kind: kind, Source: source}
}

func (a *Analyzer) hasKnown(name string) bool {
	if name == "" {
		return true
	}
	if _, ok := a.known[normalizeName(name)]; ok {
		return true
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		_, ok := a.known[normalizeName(parts[0])]
		return ok
	}
	return false
}

func (a *Analyzer) exportKnownTypes() map[string]TypeReference {
	keys := make([]string, 0, len(a.known))
	for key := range a.known {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make(map[string]TypeReference, len(keys))
	for _, key := range keys {
		ref := a.known[key]
		out[ref.Name] = ref
	}
	return out
}

var typeIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)

func extractTypeNames(typeRef string) []string {
	matches := typeIdentifierPattern.FindAllString(typeRef, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if isCollectionType(match) {
			continue
		}
		out = append(out, match)
	}
	return out
}

func isCollectionType(name string) bool {
	switch normalizeName(name) {
	case "list", "set", "map":
		return true
	default:
		return false
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (r Result) MarshalJSON() ([]byte, error) {
	type alias Result
	if len(r.Types) > 200 {
		r.Types = nil
	}
	return json.Marshal(alias(r))
}

var builtinTypes = []string{
	"void",
	"Object",
	"SObject",
	"Boolean",
	"boolean",
	"Integer",
	"Long",
	"Double",
	"Decimal",
	"String",
	"string",
	"Id",
	"Blob",
	"Date",
	"Datetime",
	"Time",
	"Type",
	"Exception",
}

var platformTypes = []string{
	"Account",
	"AggregateResult",
	"ApexPages",
	"AsyncApexJob",
	"BatchApexErrorEvent",
	"BrandTemplate",
	"Cache",
	"Cache.OrgPartition",
	"Callable",
	"Component",
	"Component.Apex.Column",
	"Component.Apex.OutputPanel",
	"Component.Apex.outputPanel",
	"CronTrigger",
	"Database",
	"Database.QueryLocator",
	"DescribeSObjectResult",
	"Dom",
	"Dom.Document",
	"Dom.XmlNode",
	"Dom.XmlNodeType",
	"EmailTemplate",
	"EntityDefinition",
	"FieldPermissions",
	"HttpCalloutMock",
	"HTTPResponse",
	"HttpResponse",
	"Iterable",
	"Matcher",
	"Messaging",
	"ObjectPermissions",
	"OrgWideEmailAddress",
	"Organization",
	"PageReference",
	"Pattern",
	"PermissionSetAssignment",
	"Profile",
	"Queueable",
	"RecentlyViewed",
	"RestRequest",
	"RestResponse",
	"Schedulable",
	"SelectOption",
	"Schema",
	"Schema.ChildRelationship",
	"Schema.DescribeSObjectResult",
	"Schema.FieldSet",
	"Schema.RecordTypeInfo",
	"Schema.SObjectField",
	"Schema.SObjectType",
	"SObjectField",
	"SObjectType",
	"StaticResource",
	"System",
	"Test",
	"TriggerOperation",
	"User",
	"UserLicense",
	"VisualEditor",
	"VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows",
}
