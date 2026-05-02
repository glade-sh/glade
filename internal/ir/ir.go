package ir

type Program struct {
	Instructions []Instruction `json:"instructions"`
}

type Instruction struct {
	Op         Op            `json:"op"`
	Type       string        `json:"type,omitempty"`
	CatchTypes []string      `json:"catchTypes,omitempty"`
	Name       string        `json:"name,omitempty"`
	Expr       Expr          `json:"expr,omitempty"`
	Init       *Instruction  `json:"init,omitempty"`
	Update     *Instruction  `json:"update,omitempty"`
	Then       []Instruction `json:"then,omitempty"`
	Else       []Instruction `json:"else,omitempty"`
	Catch      []Instruction `json:"catch,omitempty"`
	Finally    []Instruction `json:"finally,omitempty"`
	Cases      []SwitchCase  `json:"cases,omitempty"`
	Pos        int           `json:"pos,omitempty"`
}

type Op string

const (
	OpDeclare  Op = "declare"
	OpAssign   Op = "assign"
	OpExpr     Op = "expr"
	OpReturn   Op = "return"
	OpIf       Op = "if"
	OpWhile    Op = "while"
	OpDoWhile  Op = "doWhile"
	OpFor      Op = "for"
	OpForEach  Op = "forEach"
	OpBreak    Op = "break"
	OpContinue Op = "continue"
	OpThrow    Op = "throw"
	OpTry      Op = "try"
	OpSwitch   Op = "switch"
	OpRunAs    Op = "runAs"
	OpDML      Op = "dml"
)

type Expr struct {
	Kind      ExprKind   `json:"kind"`
	Value     string     `json:"value,omitempty"`
	Name      string     `json:"name,omitempty"`
	Callee    string     `json:"callee,omitempty"`
	Operator  string     `json:"operator,omitempty"`
	Args      []Expr     `json:"args,omitempty"`
	NamedArgs []NamedArg `json:"namedArgs,omitempty"`
	Left      *Expr      `json:"left,omitempty"`
	Right     *Expr      `json:"right,omitempty"`
}

type NamedArg struct {
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}

type SwitchCase struct {
	Exprs []Expr        `json:"exprs,omitempty"`
	Body  []Instruction `json:"body,omitempty"`
	Else  bool          `json:"else,omitempty"`
	Pos   int           `json:"pos,omitempty"`
}

type ExprKind string

const (
	ExprLiteral  ExprKind = "literal"
	ExprVariable ExprKind = "variable"
	ExprCall     ExprKind = "call"
	ExprUnary    ExprKind = "unary"
	ExprBinary   ExprKind = "binary"
	ExprSOQL     ExprKind = "soql"
)
