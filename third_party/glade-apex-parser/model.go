package apexast

type File struct {
	Path         string        `json:"path"`
	Kind         FileKind      `json:"kind"`
	Declarations []Declaration `json:"declarations"`
	Diagnostics  []Diagnostic  `json:"diagnostics,omitempty"`
}

type ASTFile struct {
	Path        string       `json:"path"`
	Nodes       []ASTNode    `json:"nodes"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type ASTNode struct {
	Kind     string    `json:"kind"`
	Range    Range     `json:"range"`
	Children []ASTNode `json:"children,omitempty"`
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
	Kind           DeclarationKind `json:"kind"`
	Name           string          `json:"name,omitempty"`
	Type           string          `json:"type,omitempty"`
	Modifiers      []string        `json:"modifiers,omitempty"`
	Annotations    []Annotation    `json:"annotations,omitempty"`
	Parameters     []Parameter     `json:"parameters,omitempty"`
	Accessors      []Accessor      `json:"accessors,omitempty"`
	ObjectName     string          `json:"objectName,omitempty"`
	Events         []string        `json:"events,omitempty"`
	TypeParameters []string        `json:"typeParameters,omitempty"`
	HasBody        bool            `json:"hasBody,omitempty"`
	Range          Range           `json:"range"`
	Members        []Declaration   `json:"members,omitempty"`
}

type Annotation struct {
	Name      string               `json:"name"`
	Arguments []AnnotationArgument `json:"arguments,omitempty"`
	Range     Range                `json:"range"`
}

type AnnotationArgument struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value"`
	Range Range  `json:"range"`
}

type Accessor struct {
	Kind        string       `json:"kind"`
	Modifiers   []string     `json:"modifiers,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Range       Range        `json:"range"`
	HasBody     bool         `json:"hasBody,omitempty"`
}

type Parameter struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Modifiers   []string     `json:"modifiers,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Range       Range        `json:"range"`
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
	DeclarationInitializer DeclarationKind = "initializer"
)

type Result struct {
	Files       []File       `json:"files"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

func (r Result) HasErrors() bool {
	for _, diag := range r.Diagnostics {
		if diag.Severity == Error {
			return true
		}
	}
	for _, file := range r.Files {
		if file.HasErrors() {
			return true
		}
	}
	return false
}

type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code,omitempty"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Range    *Range   `json:"range,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Offset int `json:"offset"`
}

func (f File) HasErrors() bool {
	for _, diag := range f.Diagnostics {
		if diag.Severity == Error {
			return true
		}
	}
	return false
}
