package visualforce

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

func pageEscapesLiteralAmpersands(options VisualforcePageHeaderOptions, renderAs string) bool {
	if renderAs != "" && !strings.EqualFold(renderAs, "pdf") {
		return false
	}
	mediaType, _, _ := strings.Cut(options.ContentType, ";")
	mediaType = strings.TrimSpace(mediaType)
	return mediaType == "" || strings.EqualFold(mediaType, "text/html")
}

// FlowInputBindingError describes an input conversion failure before a Flow
// starts. A hosted interview is still a separate runtime boundary.
type FlowInputBindingError struct {
	Variable string
}

func (e *FlowInputBindingError) Error() string {
	return fmt.Sprintf("Unable to set value for variable '%s'.", e.Variable)
}

var flowNumberLiteral = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func validateLiteralFlowInputs(node *MarkupNode, p project.Project) error {
	if node == nil || node.Type != MarkupNodeElement || !alternativeLiteralNodeRendered(node) {
		return nil
	}
	if strings.EqualFold(node.Namespace, "flow") && strings.EqualFold(node.Name, "interview") {
		name := node.Attribute("name")
		if strings.Contains(name, "{!") {
			return nil
		}
		definition, err := loadEmbeddingFlow(p, strings.TrimSpace(name))
		if err != nil || definition == nil {
			return err
		}
		for _, param := range node.Children {
			if param.Type != MarkupNodeElement || !strings.EqualFold(param.Namespace, "apex") ||
				!strings.EqualFold(param.Name, "param") || !alternativeLiteralNodeRendered(param) {
				continue
			}
			value, present := param.Attributes["value"]
			value = strings.TrimSpace(value)
			if !present || value == "" || strings.Contains(value, "{!") {
				continue // Nulls, expressions and scoped values retain their binding path.
			}
			for _, variable := range definition.Variables {
				if variable.Name == param.Attribute("name") && !variable.IsCollection &&
					strings.EqualFold(variable.DataType, "Number") && !flowNumberLiteral.MatchString(value) {
					return &FlowInputBindingError{Variable: variable.Name}
				}
			}
		}
		return nil
	}
	// Traverse page and literal HTML containers only. Repeats, forms, custom
	// components and dynamic rendered expressions establish other binding scopes.
	if node.Namespace != "" && !isApexPageNode(node) {
		return nil
	}
	for _, child := range node.Children {
		if err := validateLiteralFlowInputs(child, p); err != nil {
			return err
		}
	}
	return nil
}

func alternativeLiteralNodeRendered(node *MarkupNode) bool {
	value, present := node.Attributes["rendered"]
	if !present || node.Namespace == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on":
		return true
	default:
		return false
	}
}
