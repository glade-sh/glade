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
	file := apexast.ParseSourceAST(path, source)
	marks := &lineSet{lines: map[int]struct{}{}}
	w := walker{marks: marks, source: source}
	for _, node := range file.Nodes {
		switch node.Kind {
		case "class_declaration":
			if w.isTestClass(node) {
				return nil
			}
			w.classBody(node)
		case "trigger_declaration":
			for _, child := range node.Children {
				if child.Kind == "trigger_body" {
					w.statements(child.Children)
				}
			}
		case "interface_declaration", "enum_declaration":
			// Interfaces and enums have no executable lines.
		}
	}
	return marks.sorted()
}

type lineSet struct{ lines map[int]struct{} }

func (s *lineSet) mark(line int) {
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
	marks  *lineSet
	source string
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

func (w *walker) classBody(class apexast.ASTNode) {
	for _, child := range class.Children {
		if child.Kind == "class_body" {
			w.members(child.Children)
		}
	}
}

func (w *walker) members(nodes []apexast.ASTNode) {
	for _, n := range nodes {
		switch n.Kind {
		case "field_declaration":
			w.field(n)
		case "method_declaration", "constructor_declaration":
			w.method(n)
		case "static_initializer", "instance_initializer":
			for _, c := range n.Children {
				if c.Kind == "block" {
					w.statements(c.Children)
				}
			}
		case "class_declaration":
			w.classBody(n)
		}
	}
}

func (w *walker) field(n apexast.ASTNode) {
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
		// A property declaration line is executable, bodies or not.
		w.marks.mark(declarator.Range.Start.Line)
		for _, acc := range accessors.Children {
			for _, c := range acc.Children {
				if c.Kind == "block" {
					w.statements(c.Children)
				}
			}
		}
		return
	}
	if w.hasInitializer(*declarator) {
		w.marks.mark(declarator.Range.Start.Line)
		w.initializer(*declarator)
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
	w.marks.mark(line)
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
	case "line_comment", "block_comment":
	case "local_variable_declaration":
		w.marks.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			if c.Kind == "variable_declarator" {
				w.initializer(c)
			}
		}
	case "expression_statement":
		if w.isDebugOnly(n) {
			return
		}
		w.marks.mark(n.Range.Start.Line)
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
		w.marks.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			switch c.Kind {
			case "block":
				w.statement(c)
			case "catch_clause":
				w.marks.mark(c.Range.Start.Line)
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
		w.marks.mark(n.Range.Start.Line)
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
							w.marks.mark(rc.Range.Start.Line)
						}
					case "block":
						w.statement(rc)
					}
				}
			}
		}
	case "enhanced_for_statement":
		w.marks.mark(n.Range.Start.Line)
		for _, c := range n.Children {
			switch c.Kind {
			case "block":
				w.statement(c)
			case "query_expression":
				// A SOQL for loop iterates on the line that closes the query.
				w.marks.mark(c.Range.Start.Line)
				w.marks.mark(c.Range.End.Line)
			default:
				w.expr(c)
			}
		}
	default:
		// if, for, while, while, return, throw, run-as and any
		// statement shape no row has described: the start line, the
		// expression lines, and nested statements.
		w.marks.mark(n.Range.Start.Line)
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
		w.marks.mark(n.Range.Start.Line)
		return
	}
	if w.isConstant(n) {
		return
	}
	if len(n.Children) == 0 {
		w.marks.mark(n.Range.Start.Line)
		return
	}
	switch n.Kind {
	case "method_invocation", "object_creation_expression":
		w.marks.mark(n.Range.Start.Line)
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
