package ir

type Program struct {
	Instructions []Instruction `json:"instructions"`
}

type Instruction struct {
	Op   Op     `json:"op"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
	Expr Expr   `json:"expr,omitempty"`
	Pos  int    `json:"pos,omitempty"`
}

type Op string

const (
	OpDeclare Op = "declare"
	OpAssign  Op = "assign"
	OpExpr    Op = "expr"
)

type Expr struct {
	Kind     ExprKind `json:"kind"`
	Value    string   `json:"value,omitempty"`
	Name     string   `json:"name,omitempty"`
	Callee   string   `json:"callee,omitempty"`
	Operator string   `json:"operator,omitempty"`
	Args     []Expr   `json:"args,omitempty"`
	Left     *Expr    `json:"left,omitempty"`
	Right    *Expr    `json:"right,omitempty"`
}

type ExprKind string

const (
	ExprLiteral  ExprKind = "literal"
	ExprVariable ExprKind = "variable"
	ExprCall     ExprKind = "call"
	ExprUnary    ExprKind = "unary"
	ExprBinary   ExprKind = "binary"
)
