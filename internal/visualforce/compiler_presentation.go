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
		tag := presentationCompileTag(token.Data)
		if tag == "" {
			continue
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" in %s at line %d column %d", sourceName, line, column)
		if sourceName == "" {
			location = fmt.Sprintf(" at line %d column %d", line, column)
		}
		if duplicate := presentationDuplicateAttribute(startTag); duplicate != "" {
			return fmt.Errorf("Attribute %q was already specified for element %q.", duplicate, token.Data)
		}
		for _, a := range token.Attr {
			key := strings.ToLower(a.Key)
			unsupported := !strings.HasPrefix(key, "html-") && !presentationAttributeSupported(tag, key)
			switch tag {
			case "pageBlock", "pageBlockSection", "pageBlockSectionItem":
				unsupported = unsupported || key == "styleclass" || key == "style"
			case "panelGroup":
				unsupported = unsupported || key == "title" || key == "onclick"
			}
			if unsupported {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", key, tag, location)
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
	var expressions []string
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
	if controller == "" || len(expressions) == 0 {
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
			for _, value := range expressions {
				property := strings.TrimSpace(value[2 : len(value)-1])
				for _, member := range declaration.Members {
					matched := member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 && strings.EqualFold(member.Name, "get"+property) || (member.Kind == apexast.DeclarationProperty || member.Kind == apexast.DeclarationField) && strings.EqualFold(member.Name, property)
					if matched && strings.EqualFold(member.Type, "Integer") {
						return fmt.Errorf("Cannot convert the value of '%s' to the expected type.", value)
					}
				}
			}
		}
	}
	return nil
}
