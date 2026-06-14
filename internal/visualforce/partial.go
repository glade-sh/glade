package visualforce

import (
	"bytes"
	"fmt"
	"strings"

	nethtml "golang.org/x/net/html"
)

type PartialResponse struct {
	Targets   map[string]string `json:"targets"`
	ViewState string            `json:"viewState"`
	Messages  []string          `json:"messages,omitempty"`
	Redirect  string            `json:"redirect,omitempty"`
}

func NewPartialResponse(renderedHTML, viewState string, rerenderIDs []string) PartialResponse {
	targets, messages := RenderPartialTargetsWithDiagnostics(renderedHTML, rerenderIDs)
	return PartialResponse{
		Targets:   targets,
		ViewState: viewState,
		Messages:  messages,
	}
}

func RenderPartialTargets(renderedHTML string, rerenderIDs []string) map[string]string {
	targets, _ := RenderPartialTargetsWithDiagnostics(renderedHTML, rerenderIDs)
	return targets
}

func RenderPartialTargetsWithDiagnostics(renderedHTML string, rerenderIDs []string) (map[string]string, []string) {
	targets := make(map[string]string)
	if len(rerenderIDs) == 0 {
		return targets, nil
	}
	messages := []string{}
	doc, err := nethtml.Parse(strings.NewReader(renderedHTML))
	if err != nil {
		for _, id := range rerenderIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				targets[id] = addPartialDiagnostic(&messages, id, "rendered HTML could not be parsed")
			}
		}
		return targets, messages
	}
	for _, id := range rerenderIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		node := findPartialTargetNode(doc, id)
		if node == nil {
			targets[id] = addPartialDiagnostic(&messages, id, "element id not found")
			continue
		}
		targets[id] = renderPartialNode(node)
	}
	return targets, messages
}

func ParseRerenderTargets(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func findPartialTargetNode(node *nethtml.Node, id string) *nethtml.Node {
	candidates := partialTargetCandidates(id)
	var walk func(*nethtml.Node) *nethtml.Node
	walk = func(current *nethtml.Node) *nethtml.Node {
		if current == nil {
			return nil
		}
		if current.Type == nethtml.ElementNode && partialNodeMatches(current, candidates) {
			return current
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			if found := walk(child); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(node)
}

func partialTargetCandidates(id string) map[string]bool {
	out := map[string]bool{id: true}
	if idx := strings.LastIndex(id, ":"); idx >= 0 && idx+1 < len(id) {
		out[id[idx+1:]] = true
	}
	return out
}

func partialNodeMatches(node *nethtml.Node, candidates map[string]bool) bool {
	for _, attr := range node.Attr {
		if (attr.Key == "id" || attr.Key == "data-rerender") && candidates[attr.Val] {
			return true
		}
	}
	return false
}

func renderPartialNode(node *nethtml.Node) string {
	var buf bytes.Buffer
	if err := nethtml.Render(&buf, node); err != nil {
		return ""
	}
	return buf.String()
}

func partialDiagnostic(id, reason string) string {
	return fmt.Sprintf(`<!-- unsupported Visualforce partial refresh target %q: %s -->`, id, reason)
}

func addPartialDiagnostic(messages *[]string, id, reason string) string {
	diagnostic := partialDiagnostic(id, reason)
	*messages = append(*messages, diagnostic)
	return diagnostic
}
