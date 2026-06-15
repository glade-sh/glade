package codeintel

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BuildDeclarations(index typesys.Index) Graph {
	graph := NewGraph(index.Project.Root)
	for _, typ := range index.Types {
		symbol := SymbolForType(typ)
		graph.AddSymbol(symbol)
		graph.AddUse(declarationUse(symbol))
		for _, member := range typ.Members {
			memberSymbol := SymbolForMember(typ, member)
			graph.AddSymbol(memberSymbol)
			graph.AddUse(declarationUse(memberSymbol))
		}
	}
	for _, trigger := range index.Triggers {
		symbol := SymbolForTrigger(trigger)
		graph.AddSymbol(symbol)
		graph.AddUse(declarationUse(symbol))
	}
	for _, object := range index.Objects {
		symbol := SymbolForObject(object)
		graph.AddSymbol(symbol)
		graph.AddUse(declarationUse(symbol))
		for _, field := range object.Fields {
			fieldSymbol := SymbolForField(object, field)
			graph.AddSymbol(fieldSymbol)
			graph.AddUse(declarationUse(fieldSymbol))
		}
	}
	for _, record := range index.CustomMetadataRecords {
		symbol := SymbolForCustomMetadata(record)
		graph.AddSymbol(symbol)
		graph.AddUse(declarationUse(symbol))
	}
	return graph
}

func SymbolForType(typ typesys.TypeSymbol) Symbol {
	return Symbol{
		ID:         ApexTypeID(typ.Namespace, typ.Name),
		Kind:       SymbolApexType,
		Name:       typ.Name,
		Namespace:  typ.Namespace,
		File:       typ.File,
		Range:      typ.Range,
		Dependency: typ.Dependency,
		Artifact:   typ.Artifact,
		Metadata: map[string]string{
			"declarationKind": string(typ.Kind),
			"sourceRoot":      typ.SourceRoot,
			"version":         typ.Version,
		},
	}
}

func SymbolForMember(typ typesys.TypeSymbol, member typesys.MemberSymbol) Symbol {
	signature := memberSignature(member)
	return Symbol{
		ID:         ApexMemberID(typ.Namespace, typ.Name, string(member.Kind), member.Name, signature),
		Kind:       SymbolApexMember,
		Name:       member.Name,
		Container:  ApexTypeID(typ.Namespace, typ.Name),
		Namespace:  typ.Namespace,
		Type:       member.Type,
		Signature:  signature,
		File:       typ.File,
		Range:      member.Range,
		Dependency: typ.Dependency,
		Artifact:   typ.Artifact,
		Metadata: map[string]string{
			"declarationKind": string(member.Kind),
			"owner":           typ.Name,
		},
	}
}

func SymbolForTrigger(trigger typesys.TriggerSymbol) Symbol {
	return Symbol{
		ID:         TriggerID(trigger.Namespace, trigger.Name),
		Kind:       SymbolTrigger,
		Name:       trigger.Name,
		Namespace:  trigger.Namespace,
		Type:       trigger.ObjectName,
		File:       trigger.File,
		Range:      trigger.Range,
		Dependency: trigger.Dependency,
		Metadata: map[string]string{
			"object": trigger.ObjectName,
			"events": strings.Join(trigger.Events, ","),
		},
	}
}

func SymbolForObject(object schema.Object) Symbol {
	return Symbol{
		ID:   SObjectID(object.Name),
		Kind: SymbolSObject,
		Name: object.Name,
		Metadata: map[string]string{
			"label":              object.Label,
			"pluralLabel":        object.PluralLabel,
			"sharingModel":       object.SharingModel,
			"customSettingsType": object.CustomSettingsType,
		},
	}
}

func SymbolForField(object schema.Object, field schema.Field) Symbol {
	return Symbol{
		ID:        SObjectFieldID(object.Name, field.Name),
		Kind:      SymbolSObjectField,
		Name:      field.Name,
		Container: SObjectID(object.Name),
		Type:      field.Type,
		Metadata: map[string]string{
			"object":                object.Name,
			"label":                 field.Label,
			"relationshipName":      field.RelationshipName,
			"childRelationshipName": field.ChildRelationshipName,
			"referenceTo":           strings.Join(field.ReferenceTo, ","),
		},
	}
}

func SymbolForCustomMetadata(record schema.CustomMetadataRecord) Symbol {
	return Symbol{
		ID:        CustomMetadataID(record.ObjectName, record.DeveloperName),
		Kind:      SymbolCustomMetadata,
		Name:      record.FullName,
		Container: SObjectID(record.ObjectName),
		File:      record.File,
		Metadata: map[string]string{
			"object":        record.ObjectName,
			"developerName": record.DeveloperName,
			"label":         record.Label,
		},
	}
}

func declarationUse(symbol Symbol) Use {
	return Use{
		SymbolID: symbol.ID,
		Kind:     UseDeclaration,
		Name:     symbol.Name,
		File:     symbol.File,
		Range:    symbol.Range,
		Context:  symbol.Container,
		Resolved: true,
	}
}

func memberSignature(member typesys.MemberSymbol) string {
	params := make([]string, 0, len(member.Parameters))
	for _, param := range member.Parameters {
		params = append(params, strings.TrimSpace(param.Type))
	}
	return strings.TrimSpace(member.Type) + "(" + strings.Join(params, ",") + ")"
}

func symbolKindFromDeclaration(kind apexast.DeclarationKind) SymbolKind {
	switch kind {
	case apexast.DeclarationTrigger:
		return SymbolTrigger
	default:
		return SymbolApexType
	}
}
