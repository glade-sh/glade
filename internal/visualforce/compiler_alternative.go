package visualforce

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"golang.org/x/net/html"
)

// Alternative rendering has additional metadata contracts beyond the general
// page/component grammar. Keep these checks separate from the structural parser
// and validate the original source before HTML recovery loses its locations.
func validateAlternativeRenderingProject(p project.Project) error {
	for _, path := range p.VisualforcePageFiles {
		if !strings.HasSuffix(strings.ToLower(path), ".page") {
			continue
		}
		if err := validateAlternativeRenderingFile(path, p); err != nil {
			return err
		}
	}
	for _, path := range p.EmailTemplateFiles {
		if !strings.HasSuffix(strings.ToLower(path), ".email") {
			continue
		}
		if err := validateAlternativeRenderingFile(path, p); err != nil {
			return err
		}
	}
	return nil
}

func validateAlternativeRenderingFile(path string, p project.Project) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(source)
	if strings.HasSuffix(strings.ToLower(path), ".email") && !strings.Contains(strings.ToLower(text), "<messaging:emailtemplate") {
		return nil // Classic text and HTML templates retain their own loader.
	}
	if err := validateSourceTagBalance(text); err != nil {
		return err
	}
	return validateAlternativeRenderingSource(text, nameFromPath(path, filepath.Ext(path)), p)
}

var alternativeNumericTemplate = regexp.MustCompile(`^\{!\s*[+-]?[0-9]+(?:\.[0-9]+)?\s*\}$`)
var alternativeEmailReference = regexp.MustCompile(`(?i)\{!\s*(recipient|relatedTo)\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`)

type embeddingFlow struct {
	Status    string `xml:"status"`
	Variables []struct {
		Name         string `xml:"name"`
		DataType     string `xml:"dataType"`
		IsCollection bool   `xml:"isCollection"`
	} `xml:"variables"`
}

func validateAlternativeRenderingSource(source, sourceName string, p project.Project) error {
	z := html.NewTokenizer(strings.NewReader(source))
	z.AllowCDATA(true)
	offset := 0
	var flow *embeddingFlow
	lightningChildren := false
	var emailAttributes map[string]string
	emailBody := false
	for {
		kind := z.Next()
		raw := string(z.Raw())
		offset += len(raw)
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return z.Err()
			}
			break
		}
		if kind == html.EndTagToken {
			name, _ := z.TagName()
			switch string(name) {
			case "flow:interview":
				flow = nil
			case "apex:includelightning":
				lightningChildren = false
			}
			continue
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		if lightningChildren {
			return fmt.Errorf("Component <apex:includeLightning> definition does not contain <apex:componentBody> so it cannot be used with any child tags.")
		}
		values := make(map[string]string, len(token.Attr))
		for _, attr := range token.Attr {
			values[strings.ToLower(attr.Key)] = attr.Val
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" in %s at line %d column %d", sourceName, line, column)
		if sourceName == "" {
			location = fmt.Sprintf(" at line %d column %d", line, column)
		}
		label := alternativeRenderingTag(token.Data)
		if label != "" {
			if token.Data == "apex:page" && presentationDuplicateAttribute(raw) == "renderas" {
				return fmt.Errorf("Attribute %q was already specified for element %q.", "renderAs", "apex:page")
			}
			namespace, local, _ := strings.Cut(label, ":")
			spec, known := StandardComponentSpec(namespace, local)
			for _, attr := range token.Attr {
				key := strings.ToLower(attr.Key)
				if known && !alternativeAttributeSupported(spec.Attributes, key) && key != "xmlns" && !strings.HasPrefix(key, "xmlns:") && !strings.HasPrefix(key, "html-") {
					return fmt.Errorf("Unsupported attribute %s in <%s>%s", key, label, location)
				}
			}
		}
		switch token.Data {
		case "apex:page":
			value := strings.TrimSpace(values["renderas"])
			if value != "" && !strings.Contains(value, "{!") && !strings.EqualFold(value, "pdf") {
				return fmt.Errorf("Unsupported value %s for <apex:page renderAs> encountered.", value)
			}
		case "apex:canvasapp":
			if alternativeNumericTemplate.MatchString(strings.TrimSpace(values["applicationname"])) {
				return fmt.Errorf("class java.math.BigDecimal cannot be cast to class java.lang.String (java.math.BigDecimal and java.lang.String are in module java.base of loader 'bootstrap')")
			}
		case "apex:includelightning":
			lightningChildren = kind == html.StartTagToken
		case "flow:interview":
			name, present := values["name"]
			if !present {
				return fmt.Errorf("Missing required attribute name in <flow:interview>%s", location)
			}
			if strings.Contains(name, "{!") {
				continue // A dynamic name remains a runtime binding.
			}
			definition, err := loadEmbeddingFlow(p, strings.TrimSpace(name))
			if err != nil {
				return err
			}
			if definition == nil || !strings.EqualFold(definition.Status, "Active") {
				return fmt.Errorf("Flow %q is not found or doesn't have an active version.", name)
			}
			if kind == html.StartTagToken {
				flow = definition
			}
		case "apex:param":
			if flow != nil {
				name, present := values["name"]
				if !present {
					clientID, err := embeddingFlowParamClientID(source, offset-len(raw))
					if err != nil {
						return err
					}
					name = clientID
				}
				if !strings.Contains(name, "{!") && !embeddingFlowHasVariable(flow, name) {
					return fmt.Errorf("No variable named %q in flow.", name)
				}
			}
		case "apex:stylesheet":
			if err := validateEmbeddingStylesheetResource(values["value"], p); err != nil {
				return err
			}
		case "messaging:emailtemplate":
			emailAttributes = values
			if _, present := values["subject"]; !present {
				return fmt.Errorf("Missing required attribute subject in <messaging:emailTemplate>%s", location)
			}
			if recipient := values["recipienttype"]; recipient != "" && !strings.Contains(recipient, "{!") && !strings.EqualFold(recipient, "Lead") && !strings.EqualFold(recipient, "Contact") && !strings.EqualFold(recipient, "User") {
				return fmt.Errorf("The template recipient type must be Lead, Contact or User, not %s.", recipient)
			}
		case "messaging:plaintextemailbody", "messaging:htmlemailbody":
			emailBody = true // Presence includes an explicitly empty/self-closing body.
		}
	}
	if emailAttributes != nil {
		if !emailBody {
			return fmt.Errorf("You must use either plainTextEmailBody or htmlEmailBody in your template.")
		}
		return validateEmbeddingEmailFields(source, emailAttributes, p)
	}
	return nil
}

func embeddingFlowParamClientID(source string, offset int) (string, error) {
	tree, err := ParseMarkupTree(source)
	if err != nil {
		return "", err
	}
	resolver := newComponentReferenceResolver(tree, nil)
	line, column := lineColumnAt(source, offset)
	var find func(*MarkupNode) string
	find = func(node *MarkupNode) string {
		if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") &&
			strings.EqualFold(node.Name, "param") && node.Line == line && node.Column == column {
			return resolver.clientIDs[node]
		}
		for _, child := range node.Children {
			if id := find(child); id != "" {
				return id
			}
		}
		return ""
	}
	if id := find(tree); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("cannot resolve Flow parameter identifier at line %d column %d", line, column)
}

func alternativeRenderingTag(name string) string {
	switch name {
	case "apex:page":
		return "apex:page"
	case "apex:canvasapp":
		return "apex:canvasApp"
	case "apex:includelightning":
		return "apex:includeLightning"
	case "flow:interview":
		return "flow:interview"
	}
	return ""
}

func alternativeAttributeSupported(attributes []string, name string) bool {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute, name) {
			return true
		}
	}
	return false
}

func loadEmbeddingFlow(p project.Project, name string) (*embeddingFlow, error) {
	for _, path := range p.FlowFiles {
		fileName := filepath.Base(path)
		fileName = strings.TrimSuffix(strings.TrimSuffix(fileName, ".flow-meta.xml"), ".flow")
		if !strings.EqualFold(fileName, name) && !(p.Namespace != "" && strings.EqualFold(p.Namespace+"__"+fileName, name)) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var definition embeddingFlow
		if err := xml.Unmarshal(data, &definition); err != nil {
			return nil, err
		}
		return &definition, nil
	}
	return nil, nil
}

func embeddingFlowHasVariable(flow *embeddingFlow, name string) bool {
	for _, variable := range flow.Variables {
		if variable.Name == name {
			return true
		}
	}
	return false
}

func validateEmbeddingStylesheetResource(value string, p project.Project) error {
	// Inspect a direct resource expression, never a URL or a quoted literal
	// which happens to contain the spelling "$Resource".
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{!") || findExpressionTemplateEnd(value, 2) != len(value)-1 {
		return nil
	}
	expression, err := parseExpression(strings.TrimSpace(value[2 : len(value)-1]))
	if err != nil {
		return nil // The expression validator retains syntax diagnostics.
	}
	var parts []string
	switch expression := expression.(type) {
	case visualforceIdentifierExpr:
		parts = expression.parts
	case identifierExpr:
		parts = expression.parts
	}
	if len(parts) < 2 || !strings.EqualFold(parts[0], "$Resource") {
		return nil
	}
	registry, err := resource.LoadProject(project.Project{
		Root: p.Root, Namespace: p.Namespace, StaticResourceFiles: p.StaticResourceFiles,
		StaticResourceMetas: p.StaticResourceMetas, ManagedPackageDependencies: p.ManagedPackageDependencies,
	})
	if err != nil {
		return err
	}
	for _, entry := range registry.StaticResources {
		if strings.EqualFold(entry.Name, parts[1]) || (entry.NamespacePrefix != "" && strings.EqualFold(entry.NamespacePrefix+"__"+entry.Name, parts[1])) {
			return nil
		}
	}
	return fmt.Errorf("Static Resource named %s does not exist. Check spelling.", parts[1])
}

func validateEmbeddingEmailFields(source string, attributes map[string]string, p project.Project) error {
	references := alternativeEmailReference.FindAllStringSubmatch(source, -1)
	if len(references) == 0 {
		return nil
	}
	objects, err := schema.LoadProject(p)
	if err != nil {
		return err
	}
	for _, reference := range references {
		objectName := attributes[strings.ToLower(reference[1])+"type"]
		if objectName == "" || strings.Contains(objectName, "{!") {
			continue
		}
		definition, _ := storage.StandardObjectDefinition(objectName)
		if definition.Fields == nil {
			definition.Fields = map[string]storage.Field{}
		}
		for _, object := range objects.Objects {
			if strings.EqualFold(object.Name, objectName) {
				for _, field := range object.Fields {
					definition.Fields[field.Name] = storage.Field{APIName: field.Name}
				}
			}
		}
		if _, found := storage.ResolveFieldName(definition, p.Namespace, reference[2]); !found {
			return fmt.Errorf("Invalid field %s for SObject %s", reference[2], objectName)
		}
	}
	return nil
}
