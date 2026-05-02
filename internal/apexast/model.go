package apexast

import "github.com/open-aer/oaer/internal/diagnostic"

type File struct {
	Path         string                  `json:"path"`
	Kind         FileKind                `json:"kind"`
	Declarations []Declaration           `json:"declarations"`
	Diagnostics  []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type FileKind string

const (
	FileKindClass     FileKind = "class"
	FileKindInterface FileKind = "interface"
	FileKindEnum      FileKind = "enum"
	FileKindTrigger   FileKind = "trigger"
	FileKindUnknown   FileKind = "unknown"
)

type Declaration struct {
	Kind       DeclarationKind  `json:"kind"`
	Name       string           `json:"name,omitempty"`
	Type       string           `json:"type,omitempty"`
	Modifiers  []string         `json:"modifiers,omitempty"`
	ObjectName string           `json:"objectName,omitempty"`
	Events     []string         `json:"events,omitempty"`
	Range      diagnostic.Range `json:"range"`
	Members    []Declaration    `json:"members,omitempty"`
}

type DeclarationKind string

const (
	DeclarationClass       DeclarationKind = "class"
	DeclarationInterface   DeclarationKind = "interface"
	DeclarationEnum        DeclarationKind = "enum"
	DeclarationTrigger     DeclarationKind = "trigger"
	DeclarationMethod      DeclarationKind = "method"
	DeclarationConstructor DeclarationKind = "constructor"
	DeclarationField       DeclarationKind = "field"
	DeclarationProperty    DeclarationKind = "property"
)

type Result struct {
	Files       []File                  `json:"files"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

func (r Result) HasErrors() bool {
	for _, diag := range r.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	for _, file := range r.Files {
		for _, diag := range file.Diagnostics {
			if diag.Severity == diagnostic.Error {
				return true
			}
		}
	}
	return false
}
