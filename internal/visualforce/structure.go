package visualforce

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateMarkupStructure enforces the local structural rules that can be
// checked from the shared Visualforce tree. ParseMarkupTree supplies the
// HTML-aware tree, so text inside script/style raw-text elements is not treated
// as nested Visualforce markup.
func validateMarkupStructure(root *MarkupNode, sourceName string) error {
	if root == nil {
		return nil
	}
	if err := validateDocumentRootWhenPresent(root, sourceName); err != nil {
		return err
	}
	// Native child_pageBlock{SectionItem,Buttons,Table}_attribute reports
	// declaration placement before the containing component's parent rule.
	if err := validateAttributeDeclarations(root, nil); err != nil {
		return err
	}
	return validateMarkupStructureChildren(root, nil, markupStructureContext{sourceName: sourceName})
}

func validateVisualforceDocumentRoot(root *MarkupNode, expected string) error {
	if root == nil || len(root.Children) != 1 || !isApexStructureTag(root.Children[0], expected) {
		return fmt.Errorf("<apex:%s> is required and must be the outermost tag in the markup at line 1 column 1", expected)
	}
	return nil
}

func validateVisualforceFileStructure(path, expected string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Visualforce source for structure validation: %w", err)
	}
	if err := validateSourceTagBalance(string(source)); err != nil {
		return err
	}
	if err := validateFormSource(string(source), nameFromPath(path, filepath.Ext(path))); err != nil {
		return err
	}
	if err := validateSelectSource(string(source), nameFromPath(path, filepath.Ext(path))); err != nil {
		return err
	}
	if err := validatePresentationSource(string(source), nameFromPath(path, filepath.Ext(path))); err != nil {
		return err
	}
	if err := validateRemoteObjectsSource(string(source), nameFromPath(path, filepath.Ext(path))); err != nil {
		return err
	}
	root, err := parseMarkupTree(string(source), nameFromPath(path, filepath.Ext(path)))
	if err != nil {
		return err
	}
	if err := validateVisualforceDocumentRoot(root, expected); err != nil {
		return err
	}
	if err := validateFormPlacement(root); err != nil {
		return err
	}
	return validateMarkupComponents(root, nil, "")
}

func validateDocumentRootWhenPresent(root *MarkupNode, sourceName string) error {
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
	if len(root.Children) != 1 || root.Children[0] != documentRoots[0] {
		if root.Children[0].Type == MarkupNodeText {
			return fmt.Errorf("Content is not allowed in prolog.")
		}
		if root.Children[0] == documentRoots[0] && len(root.Children) > 1 && root.Children[1].Type == MarkupNodeText {
			return fmt.Errorf("Content is not allowed in trailing section.")
		}
		return fmt.Errorf("The markup in the document following the root element must be well-formed.")
	}
	for _, nested := range documentRoots[1:] {
		if !isApexStructureTag(documentRoots[0], "component") || !isApexStructureTag(nested, "component") {
			if isApexStructureTag(documentRoots[0], "page") && isApexStructureTag(nested, "page") {
				return fmt.Errorf("Cyclic page or component references '/apex/%s' are not allowed", sourceName)
			}
			return fmt.Errorf("<apex:%s> cannot be used inside <apex:%s> in the markup%s", nested.Name, documentRoots[0].Name, markupDiagnosticLocation(nested, sourceName))
		}
	}
	return nil
}

func validateAttributeDeclarations(node, parent *MarkupNode) error {
	if node == nil || node.Type != MarkupNodeElement {
		return nil
	}
	if isApexStructureTag(node, "attribute") && !isApexStructureTag(parent, "component") {
		return fmt.Errorf("<apex:attribute> must be the direct child of <apex:component>")
	}
	for _, child := range node.Children {
		if err := validateAttributeDeclarations(child, node); err != nil {
			return err
		}
	}
	return nil
}

func markupDiagnosticLocation(node *MarkupNode, sourceName string) string {
	location := ""
	if sourceName != "" {
		location = " in " + sourceName
	}
	// Native nested-page/component and missing-facet-name diagnostics locate
	// the character after the element name, before attributes or the '>'.
	return fmt.Sprintf("%s at line %d column %d", location, node.NameEndLine, node.NameEndColumn)
}

type markupStructureContext struct {
	sourceName      string
	insideComponent bool
	sections        int
	blocks          int
	forms           int
	repeatParent    *MarkupNode
}

func validateMarkupStructureChildren(node, parent *MarkupNode, context markupStructureContext) error {
	if node == nil || node.Type != MarkupNodeElement {
		return nil
	}
	if isApexStructureTag(node, "componentBody") && !context.insideComponent {
		return &componentBodyOutsideError{node: node, sourceName: context.sourceName}
	}
	columnParent := parent
	if isApexStructureTag(parent, "repeat") {
		columnParent = context.repeatParent
	}
	if isApexStructureTag(node, "column") && !isApexStructureTag(columnParent, "dataTable") && !isApexStructureTag(columnParent, "pageBlockTable") {
		return fmt.Errorf("<apex:column> must be the direct child of either <apex:dataTable> or <apex:pageBlockTable>")
	}
	if isApexStructureTag(node, "pageBlockSectionItem") && context.sections == 0 {
		return fmt.Errorf("<apex:pageBlockSectionItem> tag must be between <apex:pageBlockSection>")
	}
	if isApexStructureTag(node, "pageBlockButtons") && !isApexStructureTag(parent, "pageBlock") {
		return fmt.Errorf("<apex:pageBlock> must be the direct parent of <apex:pageBlockButtons>")
	}
	if isApexStructureTag(node, "pageBlockTable") && context.blocks == 0 && context.sections == 0 {
		return fmt.Errorf("<apex:pageBlockTable> must be contained in <apex:pageBlock> or <apex:pageBlockSection>")
	}
	if isApexStructureTag(node, "form") && context.forms != 0 {
		return fmt.Errorf("'apex:form' component cannot be nested within form tags")
	}
	if isApexStructureTag(node, "facet") {
		if _, declared := node.Attributes["name"]; !declared {
			return fmt.Errorf("Missing required attribute name in <apex:facet>%s", markupDiagnosticLocation(node, context.sourceName))
		}
	}
	if (isApexStructureTag(node, "selectOption") || isApexStructureTag(node, "selectOptions")) && !validSelectOptionStructureParent(parent) {
		return structureViolation(node, fmt.Sprintf("apex:%s must be a direct child of apex:selectCheckboxes, apex:selectRadio, or apex:selectList", strings.ToLower(node.Name)))
	}
	if isApexStructureTag(node, "pageBlockSectionItem") && visibleMarkupChildCount(node) > 2 {
		return fmt.Errorf("<apex:pageBlockSectionItem> may have no more than 2 child components")
	}
	if isApexStructureTag(node, "component") && componentBodyDefinitionCount(node) > 1 {
		directBodies := 0
		for _, child := range node.Children {
			if isApexStructureTag(child, "componentBody") {
				directBodies++
			}
		}
		// Native diagnostic_body_duplicate permits repeated direct body slots.
		// Preserve the existing rejection of nested/indirect duplicate slots.
		if directBodies != componentBodyDefinitionCount(node) {
			return structureViolation(node, "apex:component allows at most one apex:componentBody")
		}
	}
	context.insideComponent = context.insideComponent || isApexStructureTag(node, "component")
	if isApexStructureTag(node, "pageBlockSection") {
		context.sections++
	}
	if isApexStructureTag(node, "pageBlock") {
		context.blocks++
	}
	if isApexStructureTag(node, "form") {
		context.forms++
	}
	// Native repeat_column_* controls permit consecutive repeats to generate
	// table columns, but reject a non-table parent or an outputPanel barrier.
	if !isApexStructureTag(node, "repeat") {
		context.repeatParent = node
	}
	for _, child := range node.Children {
		if err := validateMarkupStructureChildren(child, node, context); err != nil {
			return err
		}
	}
	return nil
}

func validateMarkupComponents(node *MarkupNode, index *Index, namespace string) error {
	if node == nil || node.Type != MarkupNodeElement {
		return nil
	}
	if node.Namespace != "" {
		if _, builtIn := StandardComponentSpec(node.Namespace, node.Name); !builtIn {
			if node.Namespace == "apex" {
				return fmt.Errorf("Unknown component %s:%s", node.Namespace, node.Name)
			}
			if index != nil {
				name := node.Namespace + "__" + node.Name
				if node.Namespace == "c" || strings.EqualFold(node.Namespace, namespace) {
					name = node.Name
				}
				if _, ok := index.Component(name); !ok {
					if node.Namespace == "c" {
						return fmt.Errorf("Component c:%s does not exist", node.Name)
					}
					return fmt.Errorf("Component /apexcomponent/%s__%s does not exist", node.Namespace, node.Name)
				}
			}
		}
	}
	for _, child := range node.Children {
		if err := validateMarkupComponents(child, index, namespace); err != nil {
			return err
		}
	}
	return nil
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
