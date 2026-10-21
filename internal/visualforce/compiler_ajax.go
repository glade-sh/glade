package visualforce

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// AJAX source contracts are captured at both API59 and API67. Keep the token
// end offset: the Metadata API locates these diagnostics after the start tag.
func validateAjaxSource(source, sourceName string) error {
	tags := map[string]string{
		"apex:actionfunction": "actionFunction", "apex:actionsupport": "actionSupport",
		"apex:actionpoller": "actionPoller", "apex:actionstatus": "actionStatus",
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
		tag := tags[token.Data]
		if tag == "" {
			continue
		}
		line, column := lineColumnAt(source, offset)
		location := ""
		if sourceName != "" {
			location = " in " + sourceName
		}
		location += fmt.Sprintf(" at line %d column %d", line, column)
		hasName := false
		interval := ""
		for _, attribute := range token.Attr {
			// The captured poller/status immediate and poller limit controls
			// are rejected regardless of their supplied values.
			unsupported := !strings.HasPrefix(attribute.Key, "html-") && !presentationAttributeSupported(tag, attribute.Key)
			unsupported = unsupported || ((tag == "actionPoller" || tag == "actionStatus") && attribute.Key == "immediate") ||
				(tag == "actionPoller" && attribute.Key == "limit")
			if unsupported {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", attribute.Key, tag, location)
			}
			hasName = hasName || attribute.Key == "name"
			if attribute.Key == "interval" {
				interval = attribute.Val
			}
		}
		if tag == "actionFunction" && !hasName {
			return fmt.Errorf("Missing required attribute name in <apex:actionFunction>%s", location)
		}
		if tag == "actionPoller" && interval != "" && !strings.Contains(interval, "{!") {
			value, err := strconv.Atoi(strings.TrimSpace(interval))
			if err != nil {
				return fmt.Errorf("Value '%s' cannot be converted from Text to int.", interval)
			}
			if value < 5 {
				return fmt.Errorf("The interval (%d) as specified for this component is less than the minimum interval allowed (5)", value)
			}
		}
	}
}

func validateAjaxPlacement(root *MarkupNode) error {
	page := visualforceControllerRoot(root)
	if page == nil {
		return nil // A custom component can use its caller's form.
	}
	var walk func(*MarkupNode, bool) error
	walk = func(node *MarkupNode, inForm bool) error {
		inForm = inForm || isApexStructureTag(node, "form")
		if !inForm && (isApexStructureTag(node, "actionFunction") || isApexStructureTag(node, "actionPoller")) {
			tag := "actionFunction"
			if node.Name == "actionpoller" {
				tag = "actionPoller"
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

func validateAjaxControllerBinding(node *MarkupNode, ctx *expressionValidationContext) error {
	if !ctx.sourceBindings || len(ctx.formControllers) == 0 || node.Namespace != "apex" ||
		(node.Name != "actionfunction" && node.Name != "actionsupport" && node.Name != "actionpoller") {
		return nil
	}
	raw := strings.TrimSpace(node.Attribute("action"))
	if strings.EqualFold(raw, "{!NULL}") {
		return nil
	}
	action := actionMethodName(raw)
	if action == "" || !visualforceControllerMethodName.MatchString(action) {
		return nil
	}
	for _, class := range ctx.formControllers {
		if method, ok := visualforceDeclaredAction(class, action, ctx.classes, map[string]bool{}); ok {
			if !visualforceControllerMemberVisible(method) {
				return fmt.Errorf("Method is not visible: [%s].%s()", class.Name, method.Name)
			}
			return nil
		}
	}
	if ctx.standardFormActions[strings.ToLower(action)] {
		return nil
	}
	return fmt.Errorf("Unknown method '%s.%s()'", ctx.formControllers[0].Name, action)
}

func ajaxFunctionAssignmentNodes(root *MarkupNode) map[*MarkupNode]bool {
	assignments := map[*MarkupNode]bool{}
	var walk func(*MarkupNode)
	walk = func(node *MarkupNode) {
		if node == nil {
			return
		}
		for _, child := range node.Children {
			if isApexStructureTag(node, "actionFunction") && isApexStructureTag(child, "param") {
				assignments[child] = true
			}
			walk(child)
		}
	}
	walk(root)
	return assignments
}
