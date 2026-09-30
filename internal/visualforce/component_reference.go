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
		for _, child := range root.Children {
			if isApexPageNode(child) {
				resolver.indexPage(child, page, pageScope)
				continue
			}
			resolver.indexNode(child, page, page, pageScope)
		}
		return resolver
	}
	resolver.indexNode(root, page, page, pageScope)
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
	for _, child := range page.Children {
		resolver.indexNode(child, componentParent, componentParent, pageScope)
	}
}

func (resolver *componentReferenceResolver) indexNode(node *MarkupNode, componentParent, namingContainer *componentReferenceNode, scope *componentReferenceScope) {
	if node == nil {
		return
	}
	resolver.scopes[node] = scope
	if node.Type != MarkupNodeElement {
		return
	}

	if strings.EqualFold(node.Namespace, "apex") {
		localID := strings.TrimSpace(node.Attribute("id"))
		if localID == "" {
			localID = namingContainer.nextGeneratedID()
		}
		ref := &componentReferenceNode{
			localID:         localID,
			clientID:        joinComponentClientID(namingContainer.clientID, localID),
			children:        map[string]*componentReferenceNode{},
			namingContainer: strings.EqualFold(node.Name, "form"),
		}
		componentParent.children[localID] = ref
		resolver.clientIDs[node] = ref.clientID

		childContainer := namingContainer
		if ref.namingContainer {
			childContainer = ref
		}
		childScope := &componentReferenceScope{resolver: resolver, container: childContainer}
		for _, child := range node.Children {
			resolver.indexNode(child, ref, childContainer, childScope)
		}
		return
	}

	// Visualforce assigns automatic component identifiers as it builds the
	// component tree. Plain markup can consume an identifier before an unnamed
	// Visualforce component, so keep the sibling sequence even though plain
	// HTML nodes are not addressable through $Component.
	if strings.TrimSpace(node.Attribute("id")) == "" {
		namingContainer.nextGeneratedID()
	}
	for _, child := range node.Children {
		resolver.indexNode(child, componentParent, namingContainer, scope)
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
