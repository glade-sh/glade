package visualforce

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"golang.org/x/net/html"
)

// Invalid repetition cases capture required data-container attributes, unsupported
// attributes and literal Integer conversion. The native opening-tag boundary
// is checked before parent placement: missing pageBlockTable value/var wins
// over its missing pageBlock parent. General structure rules are unchanged.
func validateRepetitionSource(source, sourceName string) error {
	z := html.NewTokenizer(strings.NewReader(source))
	z.AllowCDATA(true)
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
		tag := repetitionCompileTag(token.Data)
		if tag == "" {
			continue
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" at line %d column %d", line, column)
		if sourceName != "" {
			location = " in " + sourceName + location
		}
		values := map[string]string{}
		for _, attr := range token.Attr {
			values[strings.ToLower(attr.Key)] = attr.Val
		}
		if tag != "repeat" {
			for _, required := range []string{"value", "var"} {
				if _, exists := values[required]; !exists {
					return fmt.Errorf("Missing required attribute %s in <apex:%s>%s", required, tag, location)
				}
			}
		}
		spec, _ := StandardComponentSpec("apex", tag)
		for _, attr := range token.Attr {
			key := strings.ToLower(attr.Key)
			supported := strings.HasPrefix(key, "html-")
			// Preserve the existing local iteration-index extension.
			supported = supported || tag == "repeat" && key == "indexvar"
			for _, declared := range spec.Attributes {
				supported = supported || strings.EqualFold(declared, key)
			}
			if !supported {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", key, tag, location)
			}
			if (key == "rows" || key == "first") && !strings.Contains(attr.Val, "{!") && strings.TrimSpace(attr.Val) != "" {
				if _, err := strconv.Atoi(strings.TrimSpace(attr.Val)); err != nil {
					return fmt.Errorf("Value '%s' cannot be converted from Text to int.", attr.Val)
				}
			}
		}
	}
}

func repetitionCompileTag(name string) string {
	switch strings.ToLower(name) {
	case "apex:repeat":
		return "repeat"
	case "apex:datatable":
		return "dataTable"
	case "apex:pageblocktable":
		return "pageBlockTable"
	case "apex:datalist":
		return "dataList"
	}
	return ""
}

// c_invalid_repeat_missing_value does not introduce a row variable. Other
// repetitions bind it only for their children, preserving scope-leak rejection.
// c_invalid_unknown_row_field retains the SObject element type from a declared
// List<SObjectType>; primitive and unknown collections keep existing typing.
func (ctx *expressionValidationContext) repetitionVariableBinding(node *MarkupNode) (string, bool) {
	if node.Namespace != "apex" || repetitionCompileTag("apex:"+node.Name) == "" {
		return "", true
	}
	if _, exists := node.Attributes["value"]; !exists {
		return "", node.Name != "repeat"
	}
	raw := strings.TrimSpace(node.Attribute("value"))
	if !strings.HasPrefix(raw, "{!") || !strings.HasSuffix(raw, "}") {
		return "", true
	}
	name := strings.ToLower(strings.TrimSpace(raw[2 : len(raw)-1]))
	typ := strings.TrimSpace(ctx.bindings[name])
	if len(typ) <= len("List<>") || !strings.HasPrefix(strings.ToLower(typ), "list<") || !strings.HasSuffix(typ, ">") {
		return "", true
	}
	element := strings.TrimSpace(typ[len("List<") : len(typ)-1])
	if definition, known := storage.StandardObjectDefinition(element); known {
		return definition.APIName, true
	}
	return "", true
}
