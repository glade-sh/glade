package visualforce

import (
	"fmt"
	"os"
	"strings"
)

// validateMarkupStructure enforces the local structural rules that can be
// checked from the shared Visualforce tree. ParseMarkupTree supplies the
// HTML-aware tree, so text inside script/style raw-text elements is not treated
// as nested Visualforce markup.
func validateMarkupStructure(root *MarkupNode) error {
	if root == nil {
		return nil
	}
	if err := validateDocumentRootWhenPresent(root); err != nil {
		return err
	}
	return validateMarkupStructureChildren(root, nil, false)
}

func validateVisualforceDocumentRoot(root *MarkupNode, expected string) error {
	if root == nil || len(root.Children) != 1 || !isApexStructureTag(root.Children[0], expected) {
		return fmt.Errorf("Visualforce source requires exactly one apex:%s root", expected)
	}
	return nil
}

func validateVisualforceFileStructure(path, expected string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Visualforce source for structure validation: %w", err)
	}
	root, err := ParseMarkupTree(string(source))
	if err != nil {
		return fmt.Errorf("parse Visualforce source for structure validation: %w", err)
	}
	if err := validateVisualforceDocumentRoot(root, expected); err != nil {
		return structureViolation(nil, fmt.Sprintf("%s: %v", path, err))
	}
	return nil
}

func validateDocumentRootWhenPresent(root *MarkupNode) error {
	var documentRoots []*MarkupNode
	var visit func(*MarkupNode)
	visit = func(node *MarkupNode) {
		if node == nil || node.Type != MarkupNodeElement {
			return
		}
		if isApexStructureTag(node, "page") || isApexStructureTag(node, "component") {
			documentRoots = append(documentRoots, node)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, child := range root.Children {
		visit(child)
	}
	if len(documentRoots) == 0 {
		return nil // ParseMarkupTree is also used for standalone component fragments.
	}
	if len(documentRoots) != 1 || len(root.Children) != 1 || root.Children[0] != documentRoots[0] {
		return fmt.Errorf("Visualforce page/component markup requires one document root")
	}
	return nil
}

func validateMarkupStructureChildren(node, parent *MarkupNode, insideComponent bool) error {
	if node == nil || node.Type != MarkupNodeElement {
		return nil
	}
	if isApexStructureTag(node, "attribute") && !isApexStructureTag(parent, "component") {
		return structureViolation(node, "apex:attribute must be declared directly inside apex:component")
	}
	if isApexStructureTag(node, "componentBody") && !insideComponent {
		return structureViolation(node, "apex:componentBody must be inside apex:component")
	}
	if isApexStructureTag(node, "param") && !validParamStructureParent(parent) {
		return structureViolation(node, "apex:param has an unsupported parent")
	}
	if (isApexStructureTag(node, "selectOption") || isApexStructureTag(node, "selectOptions")) && !validSelectOptionStructureParent(parent) {
		return structureViolation(node, fmt.Sprintf("apex:%s must be a direct child of apex:selectCheckboxes, apex:selectRadio, or apex:selectList", strings.ToLower(node.Name)))
	}
	if isApexStructureTag(node, "pageBlockSectionItem") && visibleMarkupChildCount(node) > 2 {
		return structureViolation(node, "apex:pageBlockSectionItem supports at most two children")
	}
	if isApexStructureTag(node, "component") && componentBodyDefinitionCount(node) > 1 {
		return structureViolation(node, "apex:component allows at most one apex:componentBody")
	}
	childInsideComponent := insideComponent || isApexStructureTag(node, "component")
	for _, child := range node.Children {
		if err := validateMarkupStructureChildren(child, node, childInsideComponent); err != nil {
			return err
		}
	}
	return nil
}

func validParamStructureParent(parent *MarkupNode) bool {
	if parent == nil {
		return false
	}
	if parent.Namespace == "flow" && parent.Name == "interview" {
		return true
	}
	if parent.Namespace != "apex" {
		return false
	}
	switch parent.Name {
	case "actionfunction", "actionsupport", "commandlink", "outputlink", "outputtext":
		return true
	case "outputformat":
		// Existing local outputFormat rendering accepts param children. The
		// retained structure-source vocabulary does not list this parent, so
		// keep that known source/local discrepancy permissive rather than
		// changing existing rendering behavior here.
		return true
	default:
		return false
	}
}

func validSelectOptionStructureParent(parent *MarkupNode) bool {
	return isApexStructureTag(parent, "selectCheckboxes") ||
		isApexStructureTag(parent, "selectRadio") ||
		isApexStructureTag(parent, "selectList")
}

func visibleMarkupChildCount(node *MarkupNode) int {
	count := 0
	for _, child := range node.Children {
		if child == nil {
			continue
		}
		if child.Type == MarkupNodeElement || (child.Type == MarkupNodeText && strings.TrimSpace(child.Text) != "") {
			count++
		}
	}
	return count
}

func componentBodyDefinitionCount(node *MarkupNode) int {
	count := 0
	var visit func(*MarkupNode)
	visit = func(parent *MarkupNode) {
		for _, child := range parent.Children {
			if isApexStructureTag(child, "componentBody") {
				count++
			}
			visit(child)
		}
	}
	visit(node)
	return count
}

func isApexStructureTag(node *MarkupNode, name string) bool {
	return node != nil && node.Type == MarkupNodeElement && node.Namespace == "apex" && strings.EqualFold(node.Name, name)
}

func structureViolation(node *MarkupNode, message string) error {
	if node != nil && node.Line > 0 {
		return fmt.Errorf("%s (line %d)", message, node.Line)
	}
	return fmt.Errorf("%s", message)
}
