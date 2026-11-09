package visualforce

import (
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"golang.org/x/net/html"
)

// Native API59/67 captures reject unsupported form/input/region
// attributes before expression validation. Keep source token locations, since
// the DOM parser has already normalized names and repaired HTML placement.
func validateFormSource(source, sourceName string) error {
	tags := map[string]string{"apex:form": "form", "apex:inputtext": "inputText", "apex:actionregion": "actionRegion", "apex:commandlink": "commandLink"}
	z := html.NewTokenizer(strings.NewReader(source))
	offset := 0
	for {
		kind := z.Next()
		offset += len(z.Raw())
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		tag := tags[strings.ToLower(token.Data)]
		if tag == "" {
			continue
		}
		for _, attribute := range token.Attr {
			if tag == "commandLink" && attribute.Key != "disabled" {
				continue
			}
			if tag != "commandLink" && (strings.HasPrefix(attribute.Key, "html-") || presentationAttributeSupported(tag, attribute.Key)) {
				continue
			}
			line, column := lineColumnAt(source, offset)
			location := ""
			if sourceName != "" {
				location = " in " + sourceName
			}
			return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s at line %d column %d", attribute.Key, tag, location, line, column)
		}
	}
}

func validateFormPlacement(root *MarkupNode) error {
	page := visualforceControllerRoot(root)
	if page == nil {
		return nil // Custom components may be placed inside a caller's form.
	}
	var walk func(*MarkupNode, bool) error
	walk = func(node *MarkupNode, inForm bool) error {
		inForm = inForm || isApexStructureTag(node, "form") || mayProvideForm(node)
		if node.Namespace == "apex" && !inForm && (node.Name == "inputtext" || node.Name == "commandbutton") {
			tag := "inputText"
			if node.Name == "commandbutton" {
				tag = "commandButton"
			}
			return fmt.Errorf("<apex:%s> (under <apex:page>) must occur between <apex:form></apex:form> tags.", tag)
		}
		for _, child := range node.Children {
			if err := walk(child, inForm); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(page, false)
}

func validateFormControllerBinding(node *MarkupNode, ctx *expressionValidationContext) error {
	if !ctx.sourceBindings || node.Namespace != "apex" {
		return nil
	}
	if len(ctx.formControllers) == 0 {
		return nil
	}
	if node.Name == "commandbutton" || node.Name == "commandlink" {
		action := actionMethodName(node.Attribute("action"))
		if action == "" || !visualforceControllerMethodName.MatchString(action) {
			return nil
		}
		found := false
		for _, class := range ctx.formControllers {
			if method, ok := visualforceDeclaredAction(class, action, ctx.classes, map[string]bool{}); ok {
				if !visualforceControllerMemberVisible(method) {
					return fmt.Errorf("Method is not visible: [%s].%s()", class.Name, method.Name)
				}
				found = true
				break
			}
		}
		if !found && !ctx.standardFormActions[strings.ToLower(action)] {
			return fmt.Errorf("Unknown method '%s.%s()'", ctx.formControllers[0].Name, action)
		}
	}
	property := formInputProperty(node)
	if property == "" {
		return nil
	}
	// Unknown properties keep the shared expression diagnostic. Known readable
	// properties used as inputs must additionally have a public setter.
	for _, class := range ctx.formControllers {
		if formDeclaredReadable(class, property, ctx.classes, map[string]bool{}) {
			if !formDeclaredWritable(class, property, ctx.classes, map[string]bool{}) {
				return fmt.Errorf("Read only property '%s.%s'", class.Name, property)
			}
			break
		}
	}
	return nil
}

// Extension methods retain precedence and visibility checks. Only methods
// already declared by the shared standard-controller catalog can fall back.
// c_review_standard_extension_save captures this fallback's acceptance at
// API 59/67, with an extension that does not declare the save action.
func formStandardControllerActions(recordSet bool) map[string]bool {
	typeName := "ApexPages.StandardController"
	if recordSet {
		typeName = "ApexPages.StandardSetController"
	}
	actions := map[string]bool{}
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		if symbol.Namespace+"."+symbol.Name != typeName {
			continue
		}
		for _, member := range symbol.Members {
			if member.Kind != apexast.DeclarationMethod || len(member.Parameters) != 0 {
				continue
			}
			static := false
			for _, modifier := range member.Modifiers {
				static = static || strings.EqualFold(modifier, "static")
			}
			if !static {
				actions[strings.ToLower(member.Name)] = true
			}
		}
		break
	}
	return actions
}

func formDeclaredReadable(class apexast.Declaration, name string, classes map[string]apexast.Declaration, seen map[string]bool) bool {
	key := strings.ToLower(class.Name)
	if seen[key] {
		return false
	}
	seen[key] = true
	for _, member := range class.Members {
		if !visualforceControllerMemberVisible(member) {
			continue
		}
		if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, "get"+name) && len(member.Parameters) == 0 {
			return true
		}
		if strings.EqualFold(member.Name, name) {
			if member.Kind == apexast.DeclarationField {
				return true
			}
			for _, accessor := range member.Accessors {
				if strings.EqualFold(accessor.Kind, "get") {
					return true
				}
			}
		}
	}
	if parent, ok := classes[strings.ToLower(class.SuperClass)]; ok {
		return formDeclaredReadable(parent, name, classes, seen)
	}
	return false
}

func formDeclaredWritable(class apexast.Declaration, name string, classes map[string]apexast.Declaration, seen map[string]bool) bool {
	key := strings.ToLower(class.Name)
	if seen[key] {
		return false
	}
	seen[key] = true
	for _, member := range class.Members {
		if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, "set"+name) && len(member.Parameters) == 1 && visualforceControllerMemberVisible(member) {
			return true
		}
		if strings.EqualFold(member.Name, name) {
			if member.Kind == apexast.DeclarationField && visualforceControllerMemberVisible(member) {
				return true
			}
			if member.Kind == apexast.DeclarationProperty && visualforceControllerMemberVisible(member) {
				for _, accessor := range member.Accessors {
					if strings.EqualFold(accessor.Kind, "set") {
						visible := true
						for _, modifier := range accessor.Modifiers {
							visible = visible && modifier != "private" && modifier != "protected"
						}
						return visible
					}
				}
			}
		}
	}
	if parent, ok := classes[strings.ToLower(class.SuperClass)]; ok {
		return formDeclaredWritable(parent, name, classes, seen)
	}
	return false
}

// mayProvideForm reports an element whose body is rendered inside markup this
// file does not contain: the body of a custom component (its definition may
// wrap <apex:componentBody/> in a form) and an <apex:define> (its template may
// wrap <apex:insert/> in one). Native rows (vf_corpus_shapes) accept a button
// or action function there. Without the component and template sources the
// placement check stays lenient rather than rejecting a page that deploys.
func mayProvideForm(node *MarkupNode) bool {
	if node.Type != MarkupNodeElement {
		return false
	}
	if node.Namespace == "apex" {
		return node.Name == "define"
	}
	return node.Namespace != ""
}

// Native input components bind to one controller property or method. A literal,
// empty or mixed value is a compile error. The tokenizer keeps the opening-tag
// boundary the diagnostic location uses.
func validateInputValueSource(source, sourceName string) error {
	tags := map[string]string{
		"apex:inputtext": "inputText", "apex:inputtextarea": "inputTextarea",
		"apex:inputsecret": "inputSecret", "apex:inputcheckbox": "inputCheckbox", "apex:selectlist": "selectList",
		// Rows IdLInputHiddenLit, IdLInputHiddenNoId, IdLInputHiddenQuote.
		"apex:inputhidden": "inputHidden",
	}
	z := html.NewTokenizer(strings.NewReader(source))
	offset := 0
	for {
		kind := z.Next()
		offset += len(z.Raw())
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		tag := tags[strings.ToLower(token.Data)]
		if tag == "" {
			continue
		}
		for _, attribute := range token.Attr {
			if !strings.EqualFold(attribute.Key, "value") {
				continue
			}
			value := attribute.Val
			if strings.HasPrefix(value, "{!") && findExpressionTemplateEnd(value, 2) == len(value)-1 {
				break
			}
			// Rows InputTextWs*, SelectListWs* and SelectListTwoExpr: spaces around a
			// single expression fail with the expression as an unknown property, and
			// adjacent expressions in a selectList fail the same way as #{...}.
			if trimmed := strings.TrimSpace(value); trimmed != value && strings.HasPrefix(trimmed, "{!") && findExpressionTemplateEnd(trimmed, 2) == len(trimmed)-1 {
				return fmt.Errorf("Unknown property '%s'", strings.TrimSpace(trimmed[2:len(trimmed)-1]))
			}
			if tag == "selectList" && onlyExpressions(value) {
				return fmt.Errorf("Unknown property '%s'", strings.ReplaceAll(value, "{!", "#{"))
			}
			line, column := lineColumnAt(source, offset)
			location := fmt.Sprintf(" at line %d column %d", line, column)
			if sourceName != "" {
				location = " in " + sourceName + location
			}
			if tag == "selectList" {
				return fmt.Errorf("Formula expression is required for attribute value in <apex:selectList>%s", location)
			}
			return fmt.Errorf("Attribute value in <apex:%s> must contain only a formula expression that resolves to a single controller variable or method%s", tag, location)
		}
	}
}

// onlyExpressions reports whether value is two or more adjacent {!...}
// expressions and nothing else.
func onlyExpressions(value string) bool {
	count := 0
	for i := 0; i < len(value); {
		if !strings.HasPrefix(value[i:], "{!") {
			return false
		}
		end := findExpressionTemplateEnd(value[i:], 2)
		if end < 0 {
			return false
		}
		i += end + 1
		count++
	}
	return count >= 2
}
