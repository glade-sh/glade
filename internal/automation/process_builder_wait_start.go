package automation

import (
	"encoding/xml"
	"regexp"
	"strings"
)

// processBuilderWaitNode preserves unknown elements so an unobserved resource
// or assignment cannot be silently discarded by the shape check.
type processBuilderWaitNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr               `xml:",any,attr"`
	Text     string                   `xml:",chardata"`
	Children []processBuilderWaitNode `xml:",any"`
}

func (n processBuilderWaitNode) one(name string) (processBuilderWaitNode, bool) {
	var found processBuilderWaitNode
	count := 0
	for _, c := range n.Children {
		if c.XMLName.Local == name {
			found = c
			count++
		}
	}
	return found, count == 1
}

func (n processBuilderWaitNode) only(names ...string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	if len(n.Attrs) != 0 || strings.TrimSpace(n.Text) != "" {
		return false
	}
	counts := map[string]int{}
	for _, c := range n.Children {
		if !allowed[c.XMLName.Local] {
			return false
		}
		counts[c.XMLName.Local]++
		if counts[c.XMLName.Local] > 1 {
			return false
		}
	}
	return true
}

func (n processBuilderWaitNode) leaf(name, want string) bool {
	c, ok := n.one(name)
	return ok && len(c.Attrs) == 0 && len(c.Children) == 0 && strings.TrimSpace(c.Text) == want
}

func processBuilderWaitIdentifier(s string) bool {
	if len(s) == 0 || len(s) > 255 {
		return false
	}
	for i, c := range []byte(s) {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

// processBuilderCapturedWaitStart verifies the captured start graph without
// relying on generated resource names. The start variable must be private and
// unread elsewhere: its only supported effect is the captured time stamp.
func processBuilderCapturedWaitStart(data []byte, current, old string) (string, bool) {
	var root processBuilderWaitNode
	if xml.Unmarshal(data, &root) != nil {
		return "", false
	}
	var assignments []processBuilderWaitNode
	for _, c := range root.Children {
		if c.XMLName.Local == "assignments" {
			assignments = append(assignments, c)
		}
	}
	if len(assignments) != 1 {
		return "", false
	}
	a := assignments[0]
	if !a.only("name", "label", "locationX", "locationY", "assignmentItems", "connector") {
		return "", false
	}
	nameNode, ok := a.one("name")
	if !ok || len(nameNode.Attrs) != 0 || len(nameNode.Children) != 0 {
		return "", false
	}
	name := strings.TrimSpace(nameNode.Text)
	if !processBuilderWaitIdentifier(name) || strings.EqualFold(name, current) || strings.EqualFold(name, old) || !root.leaf("startElementReference", name) {
		return "", false
	}
	item, ok := a.one("assignmentItems")
	if !ok || !item.only("assignToReference", "operator", "value") || !item.leaf("operator", "Assign") {
		return "", false
	}
	targetNode, ok := item.one("assignToReference")
	if !ok || len(targetNode.Attrs) != 0 || len(targetNode.Children) != 0 {
		return "", false
	}
	target := strings.TrimSpace(targetNode.Text)
	if !processBuilderWaitIdentifier(target) || strings.EqualFold(target, current) || strings.EqualFold(target, old) || strings.EqualFold(target, name) {
		return "", false
	}
	value, ok := item.one("value")
	if !ok || !value.only("elementReference") || !value.leaf("elementReference", "$Flow.CurrentDateTime") {
		return "", false
	}
	connector, ok := a.one("connector")
	if !ok || !connector.only("targetReference") {
		return "", false
	}
	next, ok := connector.one("targetReference")
	if !ok || len(next.Attrs) != 0 || len(next.Children) != 0 {
		return "", false
	}
	nextName := strings.TrimSpace(next.Text)
	decisions := 0
	for _, c := range root.Children {
		if c.XMLName.Local == "decisions" && c.leaf("name", nextName) {
			decisions++
		}
	}
	if nextName == "" || decisions != 1 {
		return "", false
	}
	variables := 0
	for _, v := range root.Children {
		if v.XMLName.Local != "variables" || !v.leaf("name", target) {
			continue
		}
		variables++
		if !v.only("name", "dataType", "isCollection", "isInput", "isOutput", "value") || !v.leaf("dataType", "DateTime") || !v.leaf("isCollection", "false") || !v.leaf("isInput", "false") || !v.leaf("isOutput", "false") {
			return "", false
		}
		d, ok := v.one("value")
		if !ok || !d.only("elementReference") || !d.leaf("elementReference", "$Flow.CurrentDateTime") {
			return "", false
		}
	}
	if variables != 1 {
		return "", false
	}
	// Only the declaration, assignment target, start connector, and assignment
	// label may mention these resources. Formula text and other references are
	// included, so the supposedly unused variable cannot affect another step.
	token := func(s string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)(^|[^a-z0-9_])` + regexp.QuoteMeta(s) + `($|[^a-z0-9_])`)
	}
	ar, vr := token(name), token(target)
	var walk func(processBuilderWaitNode, string) bool
	walk = func(n processBuilderWaitNode, path string) bool {
		path += "/" + n.XMLName.Local
		text := strings.TrimSpace(n.Text)
		if ar.MatchString(text) && !((path == "/Flow/assignments/name" || path == "/Flow/assignments/label" || path == "/Flow/startElementReference") && strings.EqualFold(text, name)) {
			return false
		}
		if vr.MatchString(text) && !((path == "/Flow/variables/name" || path == "/Flow/assignments/assignmentItems/assignToReference") && strings.EqualFold(text, target)) {
			return false
		}
		for _, child := range n.Children {
			if !walk(child, path) {
				return false
			}
		}
		return true
	}
	if !walk(root, "") {
		return "", false
	}
	return target, true
}
