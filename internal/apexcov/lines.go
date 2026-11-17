// Package apexcov models Salesforce Apex code coverage: which source lines are
// executable, how per-test line sets combine, and how percentages are formed.
//
// Every rule here is taken from native rows under
// internal/apextest/testdata/native_conformance/test_runner_coverage. A shape
// that no row covers keeps the conservative reading: its start line only.
package apexcov

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

// ExecutableLines returns the sorted 1-based source lines Salesforce counts as
// executable for one class or trigger. The union of its native covered and
// uncovered line arrays is exactly this set. A test class (annotated
// @IsTest) has no coverage and returns nil.
func ExecutableLines(path, source string) []int {
	plan := NewPlan(path, source)
	if plan == nil {
		return nil
	}
	return plan.Lines
}

// Plan is one class or trigger file's executable lines, grouped by the
// runtime event that covers them. A statement covers its start line and the
// lines of the operands it always evaluates; the right operand of && || ??
// and each ternary branch cover their own lines only when they run.
type Plan struct {
	// Name is the top-level class or trigger name; Kind is KindClass or
	// KindTrigger.
	Name string
	Kind string
	// Lines is the sorted executable set.
	Lines []int
	// Decl is the sorted subset that method, constructor and accessor entry
	// and property access cover: name lines and property declaration lines.
	// Natively a per-test row also holds the Decl lines earlier tests of the
	// same run covered.
	Decl []int

	// at maps the source position of a statement, lazy operand, catch clause,
	// switch label, or method, constructor or accessor declaration to the
	// lines its execution covers.
	at map[point][]int
	// iter maps a SOQL for-loop's position to the lines one iteration covers.
	iter map[point][]int
	// staticInit and instanceInit map a lower-case dotted class name to the
	// field initializer lines that class's static or instance initialization
	// covers.
	staticInit   map[string][]int
	instanceInit map[string][]int
	// props maps lower-case "class#property" to the property declaration
	// line an access covers.
	props map[string][]int
	// classes lists every lower-case dotted class name the file declares.
	classes []string
	exec    map[int]struct{}
}

const (
	KindClass   = "class"
	KindTrigger = "trigger"
)

type point struct{ line, col int }

// NewPlan parses one class or trigger source. It returns nil for a test
// class and for a file that declares neither a class nor a trigger.
func NewPlan(path, source string) *Plan {
	file := apexast.ParseSourceAST(path, source)
	w := &walker{
		source:       source,
		all:          newLineSet(),
		at:           map[point]*lineSet{},
		iter:         map[point]*lineSet{},
		staticInit:   map[string]*lineSet{},
		instanceInit: map[string]*lineSet{},
		props:        map[string]*lineSet{},
		decl:         newLineSet(),
	}
	plan := &Plan{}
	for _, node := range file.Nodes {
		switch node.Kind {
		case "class_declaration":
			if w.isTestClass(node) {
				return nil
			}
			plan.Kind = KindClass
			plan.Name = w.declName(node)
			w.classBody(node, plan.Name)
		case "trigger_declaration":
			plan.Kind = KindTrigger
			plan.Name = w.declName(node)
			for _, child := range node.Children {
				if child.Kind == "trigger_body" {
					w.statements(child.Children)
				}
			}
		case "interface_declaration", "enum_declaration":
			// Interfaces and enums have no executable lines.
		}
	}
	if plan.Kind == "" {
		return nil
	}
	plan.Lines = w.all.sorted()
	plan.Decl = w.decl.sorted()
	plan.exec = w.all.lines
	plan.at = freeze(w.at)
	plan.iter = freeze(w.iter)
	plan.staticInit = freeze(w.staticInit)
	plan.instanceInit = freeze(w.instanceInit)
	plan.props = freeze(w.props)
	plan.classes = w.classes
	return plan
}

// freeze keeps empty regions: a known position that covers nothing, such as
// a System.debug statement, must not fall back to its own line.
func freeze[K comparable](in map[K]*lineSet) map[K][]int {
	out := make(map[K][]int, len(in))
	for k, v := range in {
		out[k] = v.sorted()
	}
	return out
}

type lineSet struct{ lines map[int]struct{} }

func newLineSet() *lineSet { return &lineSet{lines: map[int]struct{}{}} }

func (s *lineSet) add(line int) {
	if line > 0 {
		s.lines[line] = struct{}{}
	}
}

func (s *lineSet) sorted() []int {
	out := make([]int, 0, len(s.lines))
	for line := range s.lines {
		out = append(out, line)
	}
	sort.Ints(out)
	return out
}

type walker struct {
	source string
	all    *lineSet
	// cur receives the lines marked while walking one region.
	cur *lineSet
	// eager disables lazy operand regions: a field initializer runs as one
	// unit with no operand positions.
	eager bool

	at, iter                 map[point]*lineSet
	staticInit, instanceInit map[string]*lineSet
	props                    map[string]*lineSet
	decl                     *lineSet
	classes                  []string
}

func (w *walker) mark(line int) {
	if line <= 0 {
		return
	}
	w.all.add(line)
	if w.cur != nil {
		w.cur.add(line)
	}
}

func regionIn[K comparable](m map[K]*lineSet, key K) *lineSet {
	set := m[key]
	if set == nil {
		set = newLineSet()
		m[key] = set
	}
	return set
}

// enter makes set the current region and returns the restore function.
func (w *walker) enter(set *lineSet) func() {
	previous := w.cur
	w.cur = set
	return func() { w.cur = previous }
}

func startOf(n apexast.ASTNode) point {
	return point{line: n.Range.Start.Line, col: n.Range.Start.Column}
}

func (w *walker) text(n apexast.ASTNode) string {
	// Ranges carry line and column, not offsets; callers only need short
	// spans, so slice by line.
	lines := strings.Split(w.source, "\n")
	if n.Range.Start.Line < 1 || n.Range.Start.Line > len(lines) {
		return ""
	}
	if n.Range.Start.Line == n.Range.End.Line {
		line := lines[n.Range.Start.Line-1]
		start := n.Range.Start.Column - 1
		end := n.Range.End.Column - 1
		if start < 0 || end > len(line) || start > end {
			return ""
		}
		return line[start:end]
	}
	return ""
}

// between returns the source text between two sibling nodes: the anonymous
// operator token of a binary expression.
func (w *walker) between(left, right apexast.ASTNode) string {
	start, end := left.Range.End.Offset, right.Range.Start.Offset
	if start < 0 || end > len(w.source) || start > end {
		return ""
	}
	return strings.TrimSpace(w.source[start:end])
}

func (w *walker) declName(n apexast.ASTNode) string {
	for _, child := range n.Children {
		if child.Kind == "identifier" {
			return w.text(child)
		}
	}
	return ""
}

func (w *walker) isTestClass(n apexast.ASTNode) bool {
	for _, child := range n.Children {
		if child.Kind != "modifiers" {
			continue
		}
		for _, mod := range child.Children {
			if mod.Kind != "modifier" {
				continue
			}
			for _, inner := range mod.Children {
				if inner.Kind == "annotation" && w.annotationName(inner) == "istest" {
					return true
				}
			}
		}
		for _, mod := range child.Children {
			if mod.Kind == "annotation" && w.annotationName(mod) == "istest" {
				return true
			}
		}
	}
	return false
}

func (w *walker) annotationName(n apexast.ASTNode) string {
	for _, child := range n.Children {
		if child.Kind == "identifier" {
			return strings.ToLower(w.text(child))
		}
	}
	return ""
}

func (w *walker) classBody(class apexast.ASTNode, name string) {
	key := strings.ToLower(name)
	w.classes = append(w.classes, key)
	for _, child := range class.Children {
		if child.Kind == "class_body" {
			w.members(child.Children, key)
		}
	}
}

func (w *walker) members(nodes []apexast.ASTNode, class string) {
	for _, n := range nodes {
		switch n.Kind {
		case "field_declaration":
			w.field(n, class)
		case "method_declaration", "constructor_declaration":
			w.method(n)
		case "static_initializer", "instance_initializer":
			for _, c := range n.Children {
				if c.Kind == "block" {
					w.statements(c.Children)
				}
			}
		case "class_declaration":
			w.classBody(n, class+"."+w.declName(n))
		}
	}
}

func (w *walker) isStatic(n apexast.ASTNode) bool {
	for _, child := range n.Children {
		if child.Kind != "modifiers" {
			continue
		}
		for _, mod := range child.Children {
			if mod.Kind == "modifier" && strings.EqualFold(strings.TrimSpace(w.text(mod)), "static") {
				return true
			}
		}
	}
	return false
}

func (w *walker) field(n apexast.ASTNode, class string) {
	var declarator *apexast.ASTNode
	var accessors *apexast.ASTNode
	for i := range n.Children {
		switch n.Children[i].Kind {
		case "variable_declarator":
			declarator = &n.Children[i]
		case "accessor_list":
			accessors = &n.Children[i]
		}
	}
	if declarator == nil {
		return
	}
	if accessors != nil {
		// A property declaration line is executable, bodies or not. An access
		// covers it, through an accessor body or through the property itself.
		line := declarator.Range.Start.Line
		w.mark(line)
		w.decl.add(line)
		regionIn(w.props, class+"#"+strings.ToLower(w.declName(*declarator))).add(line)
		for _, acc := range accessors.Children {
			for _, c := range acc.Children {
				if c.Kind == "block" {
					regionIn(w.at, startOf(acc)).add(line)
					w.statements(c.Children)
				}
			}
		}
		return
	}
	if w.hasInitializer(*declarator) {
		inits := w.instanceInit
		if w.isStatic(n) {
			inits = w.staticInit
		}
		restore := w.enter(regionIn(inits, class))
		eager := w.eager
		w.eager = true
		w.mark(declarator.Range.Start.Line)
		w.initializer(*declarator)
		w.eager = eager
		restore()
	}
}

func (w *walker) hasInitializer(declarator apexast.ASTNode) bool {
	for _, c := range declarator.Children {
		if c.Kind == "assignment_operator" {
			return true
		}
	}
	return false
}

func (w *walker) initializer(declarator apexast.ASTNode) {
	seen := false
	for _, c := range declarator.Children {
		if c.Kind == "assignment_operator" {
			seen = true
			continue
		}
		if seen {
			w.expr(c)
		}
	}
}

func (w *walker) method(n apexast.ASTNode) {
	var body *apexast.ASTNode
	var name *apexast.ASTNode
	for i := range n.Children {
		switch n.Children[i].Kind {
		case "block", "constructor_body":
			body = &n.Children[i]
		case "identifier":
			if name == nil {
				name = &n.Children[i]
			}
		}
	}
	if body == nil {
		return
	}
	line := n.Range.Start.Line
	if name != nil {
		line = name.Range.Start.Line
	}
	// Entering the method covers its name line.
	restore := w.enter(regionIn(w.at, startOf(n)))
	w.mark(line)
	restore()
	w.decl.add(line)
	w.statements(body.Children)
}

func (w *walker) statements(nodes []apexast.ASTNode) {
	for _, n := range nodes {
		w.statement(n)
	}
}

func (w *walker) statement(n apexast.ASTNode) {
	switch n.Kind {
	case "block", "trigger_body", "constructor_body":
		w.statements(n.Children)
		return
	case "line_comment", "block_comment":
		return
	}
	defer w.enter(regionIn(w.at, startOf(n)))()
	switch n.Kind {
	case "local_variable_declaration":
		w.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			if c.Kind == "variable_declarator" {
				w.initializer(c)
			}
		}
	case "expression_statement":
		if w.isDebugOnly(n) {
			return
		}
		w.mark(n.Range.Start.Line)
		w.exprs(n.Children)
	case "do_statement":
		for _, c := range n.Children {
			if c.Kind == "block" {
				w.statement(c)
			} else {
				w.expr(c)
			}
		}
	case "try_statement":
		w.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			switch c.Kind {
			case "block":
				w.statement(c)
			case "catch_clause":
				// Entering the catch covers its line.
				restore := w.enter(regionIn(w.at, startOf(c)))
				w.mark(c.Range.Start.Line)
				restore()
				for _, cc := range c.Children {
					if cc.Kind == "block" {
						w.statement(cc)
					}
				}
			case "finally_clause":
				for _, cc := range c.Children {
					if cc.Kind == "block" {
						w.statement(cc)
					}
				}
			}
		}
	case "switch_expression":
		w.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			if c.Kind != "switch_block" {
				w.expr(c)
				continue
			}
			for _, rule := range c.Children {
				for _, rc := range rule.Children {
					switch rc.Kind {
					case "switch_label":
						if len(rc.Children) > 0 {
							// Testing a when label covers its line.
							restore := w.enter(regionIn(w.at, startOf(rc)))
							w.mark(rc.Range.Start.Line)
							restore()
						}
					case "block":
						w.statement(rc)
					}
				}
			}
		}
	case "enhanced_for_statement":
		w.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			switch c.Kind {
			case "block":
				w.statement(c)
			case "query_expression":
				// A SOQL for loop iterates on the line that closes the query.
				w.mark(c.Range.Start.Line)
				restore := w.enter(regionIn(w.iter, startOf(n)))
				w.mark(c.Range.End.Line)
				restore()
			default:
				w.expr(c)
			}
		}
	default:
		// if, for, while, while, return, throw, run-as and any
		// statement shape no row has described: the start line, the
		// expression lines, and nested statements.
		w.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			switch c.Kind {
			case "block", "if_statement", "for_statement", "enhanced_for_statement",
				"while_statement", "do_statement", "try_statement", "switch_expression",
				"expression_statement", "local_variable_declaration", "return_statement",
				"throw_statement", "run_as_statement", "break_statement", "continue_statement":
				w.statement(c)
			default:
				w.expr(c)
			}
		}
	}
}

// isDebugOnly reports a statement that is only a System.debug call. Salesforce
// leaves its line, and its argument lines, out of the executable set.
func (w *walker) isDebugOnly(n apexast.ASTNode) bool {
	if len(n.Children) != 1 || n.Children[0].Kind != "method_invocation" {
		return false
	}
	call := n.Children[0]
	var idents []apexast.ASTNode
	for _, c := range call.Children {
		if c.Kind == "identifier" {
			idents = append(idents, c)
		}
	}
	if len(idents) < 2 {
		return false
	}
	return strings.EqualFold(w.text(idents[0]), "System") && strings.EqualFold(w.text(idents[1]), "debug")
}

func (w *walker) exprs(nodes []apexast.ASTNode) {
	for _, n := range nodes {
		w.expr(n)
	}
}

// lazyExpr walks an operand that runs only when its operator selects it, in
// a region of its own keyed by the operand's start.
func (w *walker) lazyExpr(n apexast.ASTNode) {
	if w.eager {
		w.expr(n)
		return
	}
	restore := w.enter(regionIn(w.at, startOf(n)))
	w.expr(n)
	restore()
}

// expr marks the lines of the operations inside an expression. A constant
// expression folds to one value and adds no continuation lines. A query is one
// operation however many lines it spans.
func (w *walker) expr(n apexast.ASTNode) {
	switch n.Kind {
	case "type_identifier", "generic_type", "type_arguments", "modifiers", "annotation",
		"assignment_operator", "update_operator", "line_comment", "block_comment",
		"void_type", "integral_type", "boolean_type", "array_type", "scoped_type_identifier":
		return
	case "query_expression", "soql_query_body", "sosl_query", "find_expression":
		w.mark(n.Range.Start.Line)
		return
	}
	if w.isConstant(n) {
		return
	}
	if len(n.Children) == 0 {
		w.mark(n.Range.Start.Line)
		return
	}
	switch n.Kind {
	case "method_invocation", "object_creation_expression":
		w.mark(n.Range.Start.Line)
	case "binary_expression":
		// Native rows: `&& b` is uncovered when the left side is false, and
		// `|| a` is covered when it is evaluated.
		if len(n.Children) == 2 {
			switch w.between(n.Children[0], n.Children[1]) {
			case "&&", "||", "??":
				w.expr(n.Children[0])
				w.lazyExpr(n.Children[1])
				return
			}
		}
	case "ternary_expression":
		// Native rows: the branch not taken is uncovered.
		if len(n.Children) == 3 {
			w.expr(n.Children[0])
			w.lazyExpr(n.Children[1])
			w.lazyExpr(n.Children[2])
			return
		}
	}
	for _, c := range n.Children {
		w.expr(c)
	}
}

func isLiteralKind(kind string) bool {
	switch kind {
	case "int", "decimal_integer_literal", "string_literal", "boolean", "null_literal", "decimal",
		"long", "double", "float", "character_literal", "date", "datetime", "true", "false", "null":
		return true
	}
	return false
}

// isConstant reports a literal, or an operator tree over literals only.
func (w *walker) isConstant(n apexast.ASTNode) bool {
	if isLiteralKind(n.Kind) {
		return false
	}
	switch n.Kind {
	case "binary_expression", "parenthesized_expression", "unary_expression":
		operands := 0
		for _, c := range n.Children {
			switch {
			case isLiteralKind(c.Kind):
				operands++
			case c.Kind == "binary_expression" || c.Kind == "parenthesized_expression" || c.Kind == "unary_expression":
				if !w.isConstant(c) {
					return false
				}
				operands++
			case isOperatorKind(c.Kind):
			default:
				return false
			}
		}
		return operands > 0
	}
	return false
}

func isOperatorKind(kind string) bool {
	return strings.HasSuffix(kind, "_operator") || kind == "-" || kind == "+"
}
