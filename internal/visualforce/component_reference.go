package visualforce

import (
	"strconv"
	"strings"
)

type componentReferenceResolver struct {
	root      *componentReferenceNode
	scopes    map[*MarkupNode]*componentReferenceScope
	clientIDs map[*MarkupNode]string
}

type componentReferenceNode struct {
	localID         string
	clientID        string
	children        map[string]*componentReferenceNode
	nextID          int
	namingContainer bool
}

type componentReferenceScope struct {
	resolver  *componentReferenceResolver
	container *componentReferenceNode
}

func newComponentReferenceResolver(root *MarkupNode, inherited *componentReferenceScope) *componentReferenceResolver {
	rootClientID := "j_id0"
	if inherited != nil && inherited.container != nil && strings.TrimSpace(inherited.container.clientID) != "" {
		rootClientID = inherited.container.clientID
	}
	page := &componentReferenceNode{
		localID:         rootClientID,
		clientID:        rootClientID,
		children:        map[string]*componentReferenceNode{},
		namingContainer: true,
	}
	resolver := &componentReferenceResolver{
		root:      page,
		scopes:    make(map[*MarkupNode]*componentReferenceScope),
		clientIDs: make(map[*MarkupNode]string),
	}
	pageScope := &componentReferenceScope{resolver: resolver, container: page}
	if root == nil {
		return resolver
	}
	if isApexPageNode(root) {
		resolver.indexPage(root, page, pageScope)
		return resolver
	}
	if root.Type == MarkupNodeElement && strings.EqualFold(root.Name, "_vfroot") {
		resolver.scopes[root] = pageScope
		literal := false
		for _, child := range root.Children {
			if isApexPageNode(child) {
				resolver.indexPage(child, page, pageScope)
				continue
			}
			resolver.indexNode(child, page, page, pageScope, &literal)
		}
		if literal {
			resolver.root.nextGeneratedID()
		}
		return resolver
	}
	resolver.indexChildren(&MarkupNode{Children: []*MarkupNode{root}}, page, page, pageScope)
	return resolver
}

func isApexPageNode(node *MarkupNode) bool {
	return node != nil && node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "page")
}

func (resolver *componentReferenceResolver) indexPage(page *MarkupNode, componentParent *componentReferenceNode, pageScope *componentReferenceScope) {
	if page == nil {
		return
	}
	resolver.scopes[page] = pageScope
	resolver.clientIDs[page] = componentParent.clientID
	resolver.indexChildren(page, componentParent, componentParent, pageScope)
}

// Native required-ID controls show a page-wide sequence. A contiguous run of
// plain markup is one anonymous component, including nested HTML and its text.
func (resolver *componentReferenceResolver) indexChildren(node *MarkupNode, componentParent, namingContainer *componentReferenceNode, scope *componentReferenceScope) {
	literal := false
	for _, child := range node.Children {
		resolver.indexNode(child, componentParent, namingContainer, scope, &literal)
	}
	if literal {
		resolver.root.nextGeneratedID()
	}
}

func (resolver *componentReferenceResolver) indexNode(node *MarkupNode, componentParent, namingContainer *componentReferenceNode, scope *componentReferenceScope, literal *bool) {
	if node == nil {
		return
	}
	resolver.scopes[node] = scope
	if node.Type == MarkupNodeText {
		// Native compact and indented upload controls share the same IDs.
		if strings.TrimSpace(node.Text) != "" {
			*literal = true
		}
		return
	}
	if node.Type != MarkupNodeElement {
		return
	}
	if strings.EqualFold(node.Namespace, "apex") {
		if *literal {
			resolver.root.nextGeneratedID()
			*literal = false
		}
		localID := strings.TrimSpace(node.Attribute("id"))
		if localID == "" {
			localID = resolver.root.nextGeneratedID()
		}
		ref := &componentReferenceNode{
			localID:         localID,
			clientID:        joinComponentClientID(namingContainer.clientID, localID),
			children:        map[string]*componentReferenceNode{},
			namingContainer: strings.EqualFold(node.Name, "form") || strings.EqualFold(node.Name, "repeat"),
		}
		componentParent.children[localID] = ref
		resolver.clientIDs[node] = ref.clientID
		childContainer := namingContainer
		if ref.namingContainer {
			childContainer = ref
		}
		childScope := &componentReferenceScope{resolver: resolver, container: childContainer}
		resolver.indexChildren(node, ref, childContainer, childScope)
		if strings.EqualFold(node.Name, "pageMessages") {
			// No/one/two pageMessages controls allocate 0/26/52 automatic
			// IDs: the component above plus its 25 internal components.
			resolver.root.nextID += 25
		}
		return
	}
	*literal = true // Opening HTML is part of the current literal run.
	for _, child := range node.Children {
		resolver.indexNode(child, componentParent, namingContainer, scope, literal)
	}
	if !isVoidHTMLElement(node.Name) {
		*literal = true
	}
}

func (node *componentReferenceNode) nextGeneratedID() string {
	if node == nil {
		return "j_id1"
	}
	node.nextID++
	return "j_id" + strconv.Itoa(node.nextID)
}

func joinComponentClientID(parent, local string) string {
	parent = strings.TrimSpace(parent)
	local = strings.TrimSpace(local)
	if parent == "" {
		return local
	}
	if local == "" {
		return parent
	}
	return parent + ":" + local
}

func (scope *componentReferenceScope) resolve(parts []string) (string, bool) {
	if scope == nil || scope.container == nil || len(parts) == 0 {
		return "", false
	}
	return resolveComponentReferencePath(scope.container, parts, 0)
}

func resolveComponentReferencePath(current *componentReferenceNode, parts []string, index int) (string, bool) {
	if current == nil {
		return "", false
	}
	if index >= len(parts) {
		return current.clientID, current.clientID != ""
	}
	part := strings.TrimSpace(parts[index])
	if next, ok := current.children[part]; ok {
		if clientID, found := resolveComponentReferencePath(next, parts, index+1); found {
			return clientID, true
		}
	}
	for _, child := range current.children {
		if child.namingContainer {
			continue
		}
		if clientID, found := resolveComponentReferencePath(child, parts, index); found {
			return clientID, true
		}
	}
	return "", false
}

func visualforceComponentClientID(node *MarkupNode, ctx *RenderContext) string {
	if node == nil {
		return ""
	}
	if ctx != nil && ctx.ComponentReferences != nil {
		if clientID := ctx.ComponentReferences.clientIDs[node]; clientID != "" {
			// Insert inner indices first, so an outer insertion cannot hide
			// the static prefix used by a nested repeat.
			for i := len(ctx.repeatPath) - 1; i >= 0; i-- {
				path := ctx.repeatPath[i]
				if at := strings.LastIndex(path, ":"); at >= 0 {
					prefix := path[:at] + ":"
					if strings.HasPrefix(clientID, prefix) {
						clientID = path + ":" + strings.TrimPrefix(clientID, prefix)
					}
				}
			}
			return clientID
		}
	}
	return strings.TrimSpace(node.Attribute("id"))
}

func visualforceExplicitComponentClientID(node *MarkupNode, ctx *RenderContext) string {
	if node == nil || strings.TrimSpace(node.Attribute("id")) == "" {
		return ""
	}
	return visualforceComponentClientID(node, ctx)
}

func visualforceComponentShouldRender(node *MarkupNode, ctx *RenderContext) (bool, error) {
	raw, exists := node.Attributes["rendered"]
	if !exists {
		return true, nil
	}
	value, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on":
		return true, nil
	default:
		return false, nil
	}
}
