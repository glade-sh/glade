package typesys

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
)

type Index struct {
	Project     ProjectInfo             `json:"project"`
	Types       []TypeSymbol            `json:"types"`
	Triggers    []TriggerSymbol         `json:"triggers"`
	Objects     []schema.Object         `json:"objects"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type ProjectInfo struct {
	Root             string `json:"root"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"sourceApiVersion,omitempty"`
}

type TypeSymbol struct {
	Kind      apexast.DeclarationKind `json:"kind"`
	Name      string                  `json:"name"`
	File      string                  `json:"file"`
	Modifiers []string                `json:"modifiers,omitempty"`
	IsTest    bool                    `json:"isTest,omitempty"`
	Range     diagnostic.Range        `json:"range"`
	Members   []MemberSymbol          `json:"members,omitempty"`
}

type MemberSymbol struct {
	Kind       apexast.DeclarationKind `json:"kind"`
	Name       string                  `json:"name"`
	Type       string                  `json:"type,omitempty"`
	Modifiers  []string                `json:"modifiers,omitempty"`
	Parameters []apexast.Parameter     `json:"parameters,omitempty"`
	Accessors  []apexast.Accessor      `json:"accessors,omitempty"`
	IsTest     bool                    `json:"isTest,omitempty"`
	Range      diagnostic.Range        `json:"range"`
}

type TriggerSymbol struct {
	Name       string           `json:"name"`
	ObjectName string           `json:"objectName"`
	Events     []string         `json:"events,omitempty"`
	File       string           `json:"file"`
	Range      diagnostic.Range `json:"range"`
}

func Build(p project.Project, s schema.Schema) Index {
	parser := apexast.NewParser()
	idx := Index{
		Project: ProjectInfo{
			Root:             p.Root,
			Namespace:        p.Namespace,
			SourceAPIVersion: p.SourceAPIVersion,
		},
		Objects: s.Objects,
	}
	seenTypes := make(map[string]TypeSymbol)

	for _, path := range p.ApexFiles {
		file, err := parser.ParseFile(path)
		if err != nil {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERTYPE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		idx.Diagnostics = append(idx.Diagnostics, file.Diagnostics...)
		if len(file.Diagnostics) > 0 {
			continue
		}
		for _, decl := range file.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
				sym := typeSymbolFromDeclaration(path, decl)
				key := strings.ToLower(sym.Name)
				if previous, ok := seenTypes[key]; ok {
					idx.Diagnostics = append(idx.Diagnostics, duplicateDiagnostic(sym, previous))
				} else {
					seenTypes[key] = sym
				}
				idx.Types = append(idx.Types, sym)
			case apexast.DeclarationTrigger:
				idx.Triggers = append(idx.Triggers, TriggerSymbol{
					Name:       decl.Name,
					ObjectName: decl.ObjectName,
					Events:     decl.Events,
					File:       path,
					Range:      decl.Range,
				})
			}
		}
	}

	sort.Slice(idx.Types, func(i, j int) bool {
		return idx.Types[i].Name < idx.Types[j].Name
	})
	sort.Slice(idx.Triggers, func(i, j int) bool {
		return idx.Triggers[i].Name < idx.Triggers[j].Name
	})
	return idx
}

func (idx Index) HasErrors() bool {
	for _, diag := range idx.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func typeSymbolFromDeclaration(path string, decl apexast.Declaration) TypeSymbol {
	sym := TypeSymbol{
		Kind:      decl.Kind,
		Name:      decl.Name,
		File:      path,
		Modifiers: decl.Modifiers,
		IsTest:    hasTestModifier(decl.Modifiers),
		Range:     decl.Range,
	}
	for _, member := range decl.Members {
		if member.Name == "" {
			continue
		}
		sym.Members = append(sym.Members, MemberSymbol{
			Kind:       member.Kind,
			Name:       member.Name,
			Type:       member.Type,
			Modifiers:  member.Modifiers,
			Parameters: member.Parameters,
			Accessors:  member.Accessors,
			IsTest:     hasTestModifier(member.Modifiers) || (member.Kind == apexast.DeclarationMethod && hasModifier(member.Modifiers, "testmethod")),
			Range:      member.Range,
		})
	}
	return sym
}

func hasTestModifier(modifiers []string) bool {
	for _, modifier := range modifiers {
		normalized := strings.ToLower(strings.TrimPrefix(modifier, "@"))
		if normalized == "istest" {
			return true
		}
	}
	return false
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, expected) {
			return true
		}
	}
	return false
}

func duplicateDiagnostic(current, previous TypeSymbol) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERTYPE001",
		Message:  fmt.Sprintf("duplicate top-level symbol %q; first seen in %s", current.Name, previous.File),
		File:     current.File,
		Range:    &current.Range,
	}
}
