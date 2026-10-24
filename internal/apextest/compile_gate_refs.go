package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// compileGateReferences returns the lowercased project top-level classes an
// Apex source binds to, in first-use order. It walks the parse tree and binds
// names the way Salesforce does for the dependent check:
//
//   - A type position names a project class unless an inner class of an
//     enclosing class, or one inherited from its superclasses, has that name
//     (GATE2-03, GATE2-10C). Field, parameter, return, generic, cast,
//     instanceof, class-literal and catch types all count (GATE2-08*).
//   - In an expression only the leftmost name of a dotted chain can name a
//     class, and a local, parameter, loop variable or field of that name binds
//     first (GATE2-02L, 02P, 02F, 02D). A setter's implicit parameter value
//     is such a variable, so value.x() and Value.x() inside a setter name no
//     class (GATE3-01, 01C). A method name never names a class (GATE2-02M),
//     and names compare without case (GATE2-01).
//   - String literals, so Type.forName arguments (G08), comments and
//     annotations name nothing. Inside SOQL and SOSL only bind expressions are
//     Apex, at any depth (GATE2-11A, 11B).
func compileGateReferences(path, source string, units compileGateUnits) []string {
	file := apexast.ParseSourceAST(path, source)
	walker := compileGateRefWalker{source: source, units: units, seen: make(map[string]bool)}
	for _, node := range file.Nodes {
		walker.walk(node)
	}
	return walker.refs
}

// compileGateMentions returns the lowercased project classes whose names
// appear as identifiers in source outside comments and string literals. It
// is a cheap superset of compileGateReferences that picks the files worth
// parsing.
func compileGateMentions(source string, fileOf map[string]string) map[string]bool {
	out := make(map[string]bool)
	identifier := compileGateIdentifierByte
	for i := 0; i < len(source); {
		ch := source[i]
		switch {
		case ch == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case ch == '/' && i+1 < len(source) && source[i+1] == '*':
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return out
			}
			i += end + 4
		case ch == '\'':
			i++
			for i < len(source) && source[i] != '\'' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case identifier(ch):
			j := i + 1
			for j < len(source) && identifier(source[j]) {
				j++
			}
			if ch < '0' || ch > '9' {
				if word := strings.ToLower(source[i:j]); fileOf[word] != "" {
					out[word] = true
				}
			}
			i = j
		default:
			i++
		}
	}
	return out
}

func compileGateIdentifierByte(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}

type compileGateRefWalker struct {
	source string
	units  compileGateUnits
	// schema enables object references in queries and the Schema namespace;
	// the ordinary class-dependency walk keeps its existing binding rules.
	schema bool
	refs   []string
	seen   map[string]bool
	// classes holds the enclosing type declarations, innermost last.
	classes []compileGateClassScope
	// scopes holds variable names in scope, innermost last.
	scopes []map[string]bool
	// Schema relationship receivers also need the types of lexical bindings.
	bindings        []map[string]string
	declarationType string
}

type compileGateClassScope struct {
	// name is the lowercased full type name, for example "outer.inner".
	name string
	// inner holds the lowercased names of the types declared in its body.
	inner map[string]bool
}

func (w *compileGateRefWalker) text(node apexast.ASTNode) string {
	start, end := node.Range.Start.Offset, node.Range.End.Offset
	if start < 0 || end > len(w.source) || start > end {
		return ""
	}
	return w.source[start:end]
}

func (w *compileGateRefWalker) lowerText(node apexast.ASTNode) string {
	return strings.ToLower(w.text(node))
}

func (w *compileGateRefWalker) add(name string) {
	if !w.seen[name] {
		w.seen[name] = true
		w.refs = append(w.refs, name)
	}
}

func (w *compileGateRefWalker) push(vars map[string]bool) {
	if vars == nil {
		vars = make(map[string]bool)
	}
	w.scopes = append(w.scopes, vars)
	w.bindings = append(w.bindings, make(map[string]string))
}

func (w *compileGateRefWalker) pop() {
	w.scopes = w.scopes[:len(w.scopes)-1]
	w.bindings = w.bindings[:len(w.bindings)-1]
}

func (w *compileGateRefWalker) declare(node apexast.ASTNode) {
	if len(w.scopes) == 0 {
		w.push(nil)
	}
	w.scopes[len(w.scopes)-1][w.lowerText(node)] = true
	if w.schema {
		w.bindings[len(w.bindings)-1][w.lowerText(node)] = w.declarationType
	}
}

func (w *compileGateRefWalker) isVariable(name string) bool {
	for i := len(w.scopes) - 1; i >= 0; i-- {
		if w.scopes[i][name] {
			return true
		}
	}
	return false
}

// innerType reports whether name binds to an inner type of an enclosing
// class or of one of its superclasses.
func (w *compileGateRefWalker) innerType(name string) bool {
	for i := len(w.classes) - 1; i >= 0; i-- {
		if w.classes[i].inner[name] || w.units.inheritsInnerType(w.classes[i].name, name) {
			return true
		}
	}
	return false
}

// typeName records name when it binds to a project top-level class.
func (w *compileGateRefWalker) typeName(name string) {
	if w.schema {
		if _, isClass := w.units.types[name]; isClass {
			return
		}
	}
	if w.innerType(name) {
		return
	}
	if _, ok := w.units.fileOf[name]; ok {
		w.add(name)
	}
}

// namespace reports whether name can only be the project namespace prefix.
func (w *compileGateRefWalker) namespace(name string) bool {
	if w.schema && strings.EqualFold(name, "Schema") && !w.isVariable(name) && !w.innerType(name) {
		_, isClass := w.units.types[name]
		return !isClass
	}
	if w.units.namespace == "" || !strings.EqualFold(name, w.units.namespace) || w.isVariable(name) || w.innerType(name) {
		return false
	}
	_, isClass := w.units.fileOf[name]
	return !isClass
}

func (w *compileGateRefWalker) walkChildren(node apexast.ASTNode) {
	for _, child := range node.Children {
		w.walk(child)
	}
}

func (w *compileGateRefWalker) walk(node apexast.ASTNode) {
	switch node.Kind {
	case "line_comment", "block_comment", "string_literal", "multi_line_string_literal", "annotation", "identifier":
		// A bare identifier in an expression is a variable or a method name,
		// never a class: a class reference is followed by a dot.
	case "class_declaration", "interface_declaration", "enum_declaration":
		w.walkTypeDeclaration(node)
	case "type_identifier":
		w.typeName(w.lowerText(node))
	case "scoped_type_identifier":
		w.walkScopedType(node)
	case "field_access":
		w.walkQualified(node)
	case "method_invocation":
		w.walkInvocation(node)
	case "query_expression":
		if w.schema {
			w.walkSchemaQuery(node)
		}
		w.walkQueryBinds(node)
	case "local_variable_declaration", "field_declaration", "constant_declaration", "property_declaration":
		previous := w.declarationType
		w.declarationType = w.schemaDeclarationType(node)
		w.walkChildren(node)
		w.declarationType = previous
	case "variable_declarator":
		for i, child := range node.Children {
			if i == 0 && child.Kind == "identifier" {
				w.declare(child)
				continue
			}
			w.walk(child)
		}
	case "formal_parameter", "when_sobject_type":
		previous := w.declarationType
		w.declarationType = w.schemaDeclarationType(node)
		for _, child := range node.Children {
			if child.Kind == "identifier" {
				w.declare(child)
				continue
			}
			w.walk(child)
		}
		w.declarationType = previous
	case "enhanced_for_statement":
		w.push(nil)
		previous := w.declarationType
		w.declarationType = w.schemaDeclarationType(node)
		declared := false
		for _, child := range node.Children {
			if child.Kind == "identifier" && !declared {
				w.declare(child)
				declared = true
				continue
			}
			w.walk(child)
		}
		w.pop()
		w.declarationType = previous
	case "accessor_declaration":
		// A setter has the implicit parameter value, which binds before a
		// class named Value.
		w.push(nil)
		if w.setter(node) {
			w.scopes[len(w.scopes)-1]["value"] = true
			if w.schema {
				w.bindings[len(w.bindings)-1]["value"] = w.declarationType
			}
		}
		w.walkChildren(node)
		w.pop()
	case "block", "constructor_body", "switch_rule", "catch_clause", "for_statement", "method_declaration", "constructor_declaration", "trigger_body":
		w.push(nil)
		w.walkChildren(node)
		w.pop()
	default:
		w.walkChildren(node)
	}
}

// setter reports whether a property accessor is its set accessor. The get or
// set keyword is the only token outside the accessor's modifiers, comments and
// body, so the text between those parse-tree children names it, in any case.
func (w *compileGateRefWalker) setter(node apexast.ASTNode) bool {
	var keyword strings.Builder
	at, end := node.Range.Start.Offset, node.Range.End.Offset
	for _, child := range node.Children {
		if child.Kind == "block" {
			end = child.Range.Start.Offset
			break
		}
		if child.Range.Start.Offset < at || child.Range.Start.Offset > len(w.source) {
			return false
		}
		keyword.WriteString(w.source[at:child.Range.Start.Offset])
		at = child.Range.End.Offset
	}
	if at < 0 || at > end || end > len(w.source) {
		return false
	}
	keyword.WriteString(w.source[at:end])
	return strings.EqualFold(strings.Trim(keyword.String(), " \t\r\n\f;"), "set")
}

// walkTypeDeclaration enters a class, interface or enum. Its superclass and
// interface clauses bind in the enclosing scope; its body sees its fields,
// enum constants, inner types and the fields its superclasses declare.
func (w *compileGateRefWalker) walkTypeDeclaration(node apexast.ASTNode) {
	name := ""
	var body *apexast.ASTNode
	for i := range node.Children {
		child := node.Children[i]
		switch child.Kind {
		case "identifier":
			if name == "" {
				name = w.lowerText(child)
			}
		case "class_body", "interface_body", "enum_body":
			body = &node.Children[i]
		case "modifiers":
		default:
			w.walk(child)
		}
	}
	if body == nil {
		return
	}
	full := name
	if len(w.classes) > 0 {
		full = w.classes[len(w.classes)-1].name + "." + name
	}
	scope := compileGateClassScope{name: full, inner: make(map[string]bool)}
	vars := make(map[string]bool)
	for _, member := range body.Children {
		switch member.Kind {
		case "class_declaration", "interface_declaration", "enum_declaration":
			for _, child := range member.Children {
				if child.Kind == "identifier" {
					scope.inner[w.lowerText(child)] = true
					break
				}
			}
		case "field_declaration", "constant_declaration":
			for _, child := range member.Children {
				if child.Kind == "variable_declarator" && len(child.Children) > 0 && child.Children[0].Kind == "identifier" {
					vars[w.lowerText(child.Children[0])] = true
				}
			}
		case "enum_constant":
			if len(member.Children) > 0 && member.Children[0].Kind == "identifier" {
				vars[w.lowerText(member.Children[0])] = true
			}
		}
	}
	for _, field := range w.units.inheritedFields(full) {
		vars[field] = true
	}
	w.classes = append(w.classes, scope)
	w.push(vars)
	w.walkChildren(*body)
	w.pop()
	w.classes = w.classes[:len(w.classes)-1]
}

// walkScopedType binds the leftmost part of Outer.Inner. A leading project
// namespace makes the next part the class name.
func (w *compileGateRefWalker) walkScopedType(node apexast.ASTNode) {
	var parts []apexast.ASTNode
	var collect func(apexast.ASTNode)
	collect = func(n apexast.ASTNode) {
		for _, child := range n.Children {
			switch child.Kind {
			case "type_identifier":
				parts = append(parts, child)
			case "scoped_type_identifier":
				collect(child)
			default:
				w.walk(child)
			}
		}
	}
	collect(node)
	if len(parts) == 0 {
		return
	}
	first := w.lowerText(parts[0])
	if len(parts) > 1 && w.namespace(first) {
		if _, ok := w.units.fileOf[w.lowerText(parts[1])]; ok {
			if w.schema && first != "schema" {
				w.typeName(w.lowerText(parts[1]))
			} else {
				w.add(w.lowerText(parts[1]))
			}
		}
		return
	}
	w.typeName(first)
}

// qualifier binds the object of a member access or call. Only its leftmost
// name can name a class, and a variable of that name binds first (GATE2-02D).
func (w *compileGateRefWalker) qualifier(object apexast.ASTNode) {
	switch object.Kind {
	case "identifier":
		if name := w.lowerText(object); !w.isVariable(name) {
			w.typeName(name)
		}
	case "field_access":
		w.walkQualified(object)
	default:
		w.walk(object)
	}
}

// walkQualified binds a.b.c. Members after a dot name nothing, except the
// class after a project namespace prefix.
func (w *compileGateRefWalker) walkQualified(node apexast.ASTNode) {
	if len(node.Children) == 0 {
		return
	}
	if w.schema {
		w.schemaObjectToken(node)
		w.schemaRelationshipAccess(node)
	}
	object := node.Children[0]
	if object.Kind == "identifier" && w.namespace(w.lowerText(object)) {
		for _, child := range node.Children[1:] {
			if child.Kind == "identifier" {
				if name := w.lowerText(child); w.units.fileOf[name] != "" {
					if w.schema && w.lowerText(object) != "schema" {
						w.typeName(name)
					} else {
						w.add(name)
					}
				}
				break
			}
		}
	} else {
		w.qualifier(object)
	}
	for _, child := range node.Children[1:] {
		if child.Kind != "identifier" {
			w.walk(child)
		}
	}
}

// walkInvocation binds the object of a.m(args); a bare m(args) is a method
// call (GATE2-02M).
func (w *compileGateRefWalker) walkInvocation(node apexast.ASTNode) {
	args := -1
	for i, child := range node.Children {
		if child.Kind == "argument_list" {
			args = i
		}
	}
	if args < 0 {
		w.walkChildren(node)
		return
	}
	name := args - 1
	for name >= 0 && node.Children[name].Kind == "type_arguments" {
		w.walk(node.Children[name])
		name--
	}
	if name > 0 {
		w.qualifier(node.Children[0])
	}
	for i := args; i < len(node.Children); i++ {
		w.walk(node.Children[i])
	}
}

// walkQueryBinds walks only the Apex bind expressions of a SOQL or SOSL
// literal; field, object and alias names are not Apex names.
func (w *compileGateRefWalker) walkQueryBinds(node apexast.ASTNode) {
	for _, child := range node.Children {
		if child.Kind == "bound_apex_expression" {
			w.walkChildren(child)
			continue
		}
		w.walkQueryBinds(child)
	}
}

// inheritsInnerType reports whether the project type full (lowercased), or a
// project superclass of it, declares an inner type name.
func (units compileGateUnits) inheritsInnerType(full, name string) bool {
	for depth, current := 0, full; current != "" && depth < 32; depth++ {
		if _, ok := units.types[current+"."+name]; ok {
			return true
		}
		current = units.superclass(current)
	}
	return false
}

// inheritedFields returns the lowercased field and property names that the
// project superclasses of full declare.
func (units compileGateUnits) inheritedFields(full string) []string {
	var out []string
	for depth, current := 0, units.superclass(full); current != "" && depth < 32; depth++ {
		for _, member := range units.types[current].Members {
			if member.Kind == apexast.DeclarationField || member.Kind == apexast.DeclarationProperty {
				out = append(out, strings.ToLower(member.Name))
			}
		}
		current = units.superclass(current)
	}
	return out
}

// superclass resolves the declared superclass of the project type full to a
// lowercased project type name: a sibling or enclosing inner type first, then
// a top-level class. It returns "" outside the project.
func (units compileGateUnits) superclass(full string) string {
	typ, ok := units.types[full]
	if !ok {
		return ""
	}
	super := strings.ToLower(strings.TrimSpace(typ.SuperClass))
	if super == "" {
		return ""
	}
	if ns := strings.ToLower(units.namespace); ns != "" {
		super = strings.TrimPrefix(super, ns+".")
	}
	for owner := strings.ToLower(typ.OwnerName); owner != ""; {
		if _, ok := units.types[owner+"."+super]; ok {
			return owner + "." + super
		}
		dot := strings.LastIndexByte(owner, '.')
		if dot < 0 {
			break
		}
		owner = owner[:dot]
	}
	if _, ok := units.types[super]; ok && super != full {
		return super
	}
	return ""
}

func compileGateTypeKey(typ typesys.TypeSymbol) string {
	return strings.ToLower(typ.Name)
}
