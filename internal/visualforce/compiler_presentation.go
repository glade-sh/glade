package visualforce

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"golang.org/x/net/html"
)

// These compile contracts come from API59/67 presentation controls.
// Validate source tokens before the HTML parser can erase duplicate attributes
// or repair the element tree. Diagnostics point just past the opening tag.
func validatePresentationSource(source, sourceName string) error {
	z := html.NewTokenizer(strings.NewReader(source))
	z.AllowCDATA(true)
	offset := 0
	linkDepth := 0
	for {
		kind := z.Next()
		raw := z.Raw()
		offset += len(raw)
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		}
		if kind == html.EndTagToken {
			if name, _ := z.TagName(); strings.EqualFold(string(name), "apex:outputlink") && linkDepth > 0 {
				linkDepth--
			}
			continue
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		// Tokenizer.readTag discards repeated attribute names, and Token can
		// mutate Raw's backing buffer while normalizing names and values.
		// Preserve the source spelling before asking for the HTML token.
		startTag := string(raw)
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" in %s at line %d column %d", sourceName, line, column)
		if sourceName == "" {
			location = fmt.Sprintf(" at line %d column %d", line, column)
		}
		if err := validateNativeIDAndParam(token, startTag, location, linkDepth); err != nil {
			return err
		}
		if kind == html.StartTagToken && strings.EqualFold(token.Data, "apex:outputlink") {
			linkDepth++
		}
		tag := presentationCompileTag(token.Data)
		if tag == "" {
			continue
		}
		if duplicate := presentationDuplicateAttribute(startTag); duplicate != "" {
			return fmt.Errorf("Attribute %q was already specified for element %q.", duplicate, token.Data)
		}
		for _, a := range token.Attr {
			key := strings.ToLower(a.Key)
			unsupported := !strings.HasPrefix(key, "html-") && !presentationAttributeSupported(tag, key)
			switch tag {
			case "page":
				// authorization and request context policy_requireSSL_{true,false,maybe} rejects this
				// attribute regardless of its value at API 59 and 67; other
				// unsupported page attributes stay rejected (view state view-state rows).
				unsupported = unsupported || key == "requiressl"
			case "pageBlock", "pageBlockSection", "pageBlockSectionItem":
				unsupported = unsupported || key == "styleclass" || key == "style"
			case "panelGroup":
				unsupported = unsupported || key == "title" || key == "onclick"
			case "inputField":
				unsupported = unsupported || key == "disabled" || key == "tabindex"
			}
			if unsupported {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", key, tag, location)
			}
			if (tag == "inputField" || tag == "outputField") && key == "value" &&
				(!strings.HasPrefix(a.Val, "{!") || findExpressionTemplateEnd(a.Val, 2) != len(a.Val)-1) {
				return fmt.Errorf("Attribute value in <apex:%s> must contain only a formula expression that resolves to a single controller variable or method%s", tag, location)
			}
			if strings.HasPrefix(key, "html-") {
				if tag == "panelGroup" {
					return fmt.Errorf("Component <apex:panelGroup> does not support pass-through attributes%s", location)
				}
				attrName := strings.TrimPrefix(key, "html-")
				reserved := attrName == "title" && (tag == "outputPanel" || tag == "outputText" || tag == "panelGrid" || tag == "pageBlock") || attrName == "onclick" && (tag == "outputPanel" || tag == "panelGrid" || tag == "pageBlock")
				if reserved {
					return fmt.Errorf("Cannot override attribute '%s' on component <apex:%s>%s", attrName, tag, location)
				}
			}
			if key == "columns" && (tag == "panelGrid" || tag == "pageBlockSection") && !strings.Contains(a.Val, "{!") {
				if _, err := strconv.Atoi(strings.TrimSpace(a.Val)); err != nil && strings.TrimSpace(a.Val) != "" {
					return fmt.Errorf("Value '%s' cannot be converted from Text to int.", a.Val)
				}
			}
			if tag == "page" && key == "expires" && strings.TrimSpace(a.Val) != "" && !strings.Contains(a.Val, "{!") {
				if _, err := strconv.ParseFloat(strings.TrimSpace(a.Val), 64); err != nil {
					return fmt.Errorf("Value '%s' cannot be converted from Text to Number.", a.Val)
				}
			}
		}
		if tag == "pageMessage" {
			for _, a := range token.Attr {
				if strings.EqualFold(a.Key, "strength") && pageMessageStrengthIsText(a.Val) {
					return fmt.Errorf("Wrong type for attribute <apex:pageMessage strength=\"%s\">. Expected Integer, found String", a.Val)
				}
			}
			severity := false
			for _, a := range token.Attr {
				severity = severity || strings.EqualFold(a.Key, "severity")
			}
			if !severity {
				return fmt.Errorf("Missing required attribute severity in <apex:pageMessage>%s", location)
			}
		}
		if tag == "page" {
			values := map[string]string{}
			for _, a := range token.Attr {
				values[strings.ToLower(a.Key)] = a.Val
			}
			if strings.EqualFold(values["applyhtmltag"], "false") && !strings.EqualFold(values["showheader"], "false") {
				return fmt.Errorf("Attribute 'showHeader' on component <apex:page> must be false when 'applyHtmlTag' is false.")
			}
		}
	}
}

// The tokenizer supplies the opening-tag boundary, including quoted '>'
// characters. Walk just its attributes so quoted text that resembles an
// attribute, script/style bodies, and namespace prefixes remain intact.
func presentationDuplicateAttribute(raw string) string {
	i := 1 // Skip '<' and the element name.
	for i < len(raw) && !isMarkupWhitespace(raw[i]) && raw[i] != '>' && raw[i] != '/' {
		i++
	}
	seen := map[string]bool{}
	for i < len(raw) {
		for i < len(raw) && isMarkupWhitespace(raw[i]) {
			i++
		}
		if i >= len(raw) || raw[i] == '>' || raw[i] == '/' {
			break
		}
		start := i
		for i < len(raw) && !isMarkupWhitespace(raw[i]) && raw[i] != '=' && raw[i] != '>' && raw[i] != '/' {
			i++
		}
		key := strings.ToLower(raw[start:i])
		if key == "" {
			break
		}
		if seen[key] {
			return key
		}
		seen[key] = true
		for i < len(raw) && isMarkupWhitespace(raw[i]) {
			i++
		}
		if i >= len(raw) || raw[i] != '=' {
			continue
		}
		i++
		for i < len(raw) && isMarkupWhitespace(raw[i]) {
			i++
		}
		if i < len(raw) && (raw[i] == '\'' || raw[i] == '"') {
			quote := raw[i]
			i++
			for i < len(raw) && raw[i] != quote {
				i++
			}
			if i < len(raw) {
				i++
			}
		} else {
			for i < len(raw) && !isMarkupWhitespace(raw[i]) && raw[i] != '>' {
				i++
			}
		}
	}
	return ""
}

func presentationAttributeSupported(tag, name string) bool {
	spec, ok := StandardComponentSpec("apex", tag)
	if !ok {
		return true
	}
	for _, attribute := range spec.Attributes {
		if strings.EqualFold(attribute, name) {
			return true
		}
	}
	return false
}

func presentationCompileTag(name string) string {
	switch strings.ToLower(name) {
	case "apex:page":
		return "page"
	case "apex:outputpanel":
		return "outputPanel"
	case "apex:outputtext":
		return "outputText"
	case "apex:inputfield":
		return "inputField"
	case "apex:outputfield":
		return "outputField"
	case "apex:pageblock":
		return "pageBlock"
	case "apex:pageblocksection":
		return "pageBlockSection"
	case "apex:pageblocksectionitem":
		return "pageBlockSectionItem"
	case "apex:panelgrid":
		return "panelGrid"
	case "apex:panelgroup":
		return "panelGroup"
	case "apex:chart":
		return "chart"
	case "apex:pagemessage":
		return "pageMessage"
	case "apex:outputlink":
		return "outputLink"
	}
	return ""
}

// Layout conversion rejects an Integer getter bound to rendered, while
// Boolean getters and null continue through the existing renderer path.
// Resolve the declared type from the project's actual controller source.
func validatePresentationControllerTypes(root *MarkupNode, p project.Project) error {
	if root == nil {
		return nil
	}
	controller := ""
	var expressions, strengthExpressions []string
	var visit func(*MarkupNode) error
	visit = func(n *MarkupNode) error {
		if n == nil || n.Type != MarkupNodeElement {
			return nil
		}
		if isApexStructureTag(n, "page") {
			controller = n.Attribute("controller")
		}
		if isApexStructureTag(n, "chart") && visibleMarkupChildCount(n) == 0 {
			return fmt.Errorf("Expected child components for tag 'apex:chart'")
		}
		if presentationCompileTag(n.RawName) == "pageMessage" {
			// A Boolean getter bound to strength is a compile error (row
			// PageMsgStrengthExprBool); a String one is not (PageMsgStrengthExprVar).
			value := n.Attributes["strength"]
			if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
				strengthExpressions = append(strengthExpressions, value)
			}
		}
		switch presentationCompileTag(n.RawName) {
		case "pageBlock", "pageBlockSection", "pageBlockSectionItem", "panelGrid", "panelGroup":
			value := n.Attribute("rendered")
			if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
				expressions = append(expressions, value)
			}
		}
		for _, child := range n.Children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return err
	}
	if controller == "" || len(expressions)+len(strengthExpressions) == 0 {
		return nil
	}
	parser := apexast.NewParser()
	defer parser.Close()
	for _, path := range p.ApexFiles {
		if !strings.EqualFold(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), controller) {
			continue
		}
		file, err := parser.ParseFile(path)
		if err != nil {
			return err
		}
		for _, declaration := range file.Declarations {
			if !strings.EqualFold(declaration.Name, controller) {
				continue
			}
			// A field without accessors is an unknown property, not a typed
			// value (rows ExprField*, PageMsgStrengthExprField).
			for _, value := range expressions {
				property := strings.TrimSpace(value[2 : len(value)-1])
				for _, member := range declaration.Members {
					matched := member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 && strings.EqualFold(member.Name, "get"+property) || member.Kind == apexast.DeclarationProperty && strings.EqualFold(member.Name, property)
					if matched && strings.EqualFold(member.Type, "Integer") {
						return fmt.Errorf("Cannot convert the value of '%s' to the expected type.", value)
					}
				}
			}
			for _, value := range strengthExpressions {
				property := strings.TrimSpace(value[2 : len(value)-1])
				for _, member := range declaration.Members {
					matched := member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 && strings.EqualFold(member.Name, "get"+property) || member.Kind == apexast.DeclarationProperty && strings.EqualFold(member.Name, property)
					if matched && strings.EqualFold(member.Type, "Boolean") {
						return fmt.Errorf("Wrong type for attribute <apex:pageMessage strength=\"%s\">. Expected Integer, found Boolean", value)
					}
				}
			}
		}
	}
	return nil
}

// validateNativeIDAndParam holds two compile rules every API version shares
// (rows IdExpr*, IdX*, IdEmpty* and LinkParamNoValue): an id on a standard
// component must be a non-empty literal, and an apex:param inside an
// apex:outputLink needs a value.
func validateNativeIDAndParam(token html.Token, startTag, location string, linkDepth int) error {
	if !strings.HasPrefix(strings.ToLower(token.Data), "apex:") {
		return nil
	}
	component := token.Data[len("apex:"):]
	if spec, ok := StandardComponentSpec("apex", component); ok && spec.Name != "" {
		component = strings.TrimPrefix(spec.Name, "apex:")
	} else if len(startTag) > len("<apex:") {
		component = startTag[len("<apex:"):]
		if end := strings.IndexAny(component, " \t\r\n/>"); end >= 0 {
			component = component[:end]
		}
	}
	hasValue := false
	for _, a := range token.Attr {
		switch strings.ToLower(a.Key) {
		case "id":
			if strings.EqualFold(component, "facet") {
				// Rows IdXFacet and IdLFacet: a facet takes no id, literal or not.
				return fmt.Errorf("Unsupported attribute id in <apex:facet>%s", location)
			}
			if a.Val == "" {
				// Rows IdEmpty*: an empty id fails the page's validation on every component.
				return fmt.Errorf("Failed validation: ApexPage")
			}
			// vf_whitespace_ids captures a single-space ID on these two
			// components as rejected at both supported API floors.
			if a.Val == " " && (strings.EqualFold(component, "outputText") || strings.EqualFold(component, "outputPanel")) {
				return fmt.Errorf("Failed validation: ApexPage")
			}
			if strings.Contains(a.Val, "{!") {
				return fmt.Errorf("Literal value is required for attribute id in <apex:%s>%s", component, location)
			}
		case "value":
			hasValue = true
		}
	}
	if strings.EqualFold(component, "param") && linkDepth > 0 && !hasValue {
		return fmt.Errorf("Missing required attribute value in <apex:param>%s", location)
	}
	return nil
}

// pageMessageStrengthIsText reports whether a strength attribute is text that
// the org rejects when it compiles the page (rows PageMsgStrength* of
// testdata/vf_render_markup): a literal that is not an integer, spaces
// included, or an expression that is a string constant that is not an integer. An empty
// literal compiles and fails when the page renders; an expression the org
// cannot type before the page runs is left to the renderer.
func pageMessageStrengthIsText(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		inner := value[2 : len(value)-1]
		if len(inner) > 2 && inner[0] == '\'' && inner[len(inner)-1] == '\'' && !strings.Contains(inner[1:len(inner)-1], "'") {
			// A string constant is converted to an Integer when the page
			// compiles: '4' is accepted (PageMsgStrengthExprNumText), 'x' is not.
			_, err := strconv.Atoi(inner[1 : len(inner)-1])
			return err != nil
		}
		return false
	}
	if strings.Contains(value, "{!") {
		return false
	}
	_, err := strconv.Atoi(value)
	return err != nil
}
