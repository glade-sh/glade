package apextest

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// Trigger members may be method declarations, independently of executable
// statements: BaseApexParser.g4, triggerBlockMember and triggerMemberDeclaration.
// https://github.com/apex-dev-tools/apex-parser/blob/main/antlr/BaseApexParser.g4#rows
// https://github.com/apex-dev-tools/apex-parser/blob/main/antlr/BaseApexParser.g4#L146-L153
// This separates known syntax only. The caller must retain an explicit boundary
// for reached helpers; removing a declaration does not establish its semantics.
func triggerExecutableSource(source string, trigger typesys.TriggerSymbol) (string, []string, error) {
	parsed := apexast.ParseSourceAST(trigger.File, source)
	if len(parsed.Diagnostics) != 0 {
		return source, nil, fmt.Errorf("trigger method extraction: %s", parsed.Diagnostics[0].Message)
	}
	var bodies []apexast.ASTNode
	for _, node := range parsed.Nodes {
		if node.Kind != "trigger_declaration" {
			continue
		}
		identifiers, err := triggerDirectIdentifiers(source, node)
		if err != nil {
			return source, nil, err
		}
		if len(identifiers) != 2 {
			return source, nil, fmt.Errorf("trigger method extraction: ambiguous trigger identity")
		}
		if trigger.Name != "" && !strings.EqualFold(identifiers[0], trigger.Name) {
			continue
		}
		if trigger.ObjectName != "" && !strings.EqualFold(identifiers[1], trigger.ObjectName) {
			continue
		}
		if trigger.Range.End.Offset > trigger.Range.Start.Offset &&
			(node.Range.Start.Offset != trigger.Range.Start.Offset || node.Range.End.Offset != trigger.Range.End.Offset) {
			continue
		}
		for _, child := range node.Children {
			if child.Kind == "trigger_body" {
				bodies = append(bodies, child)
			}
		}
	}
	if len(bodies) != 1 {
		return source, nil, fmt.Errorf("trigger method extraction: expected one matching trigger body, got %d", len(bodies))
	}
	body := bodies[0]
	start, end := body.Range.Start.Offset, body.Range.End.Offset
	if start < 0 || end <= start || end > len(source) || source[start] != '{' || source[end-1] != '}' {
		return source, nil, fmt.Errorf("trigger method extraction: invalid trigger body range")
	}
	if trigger.BodyRange != nil &&
		(start != trigger.BodyRange.Start.Offset || end != trigger.BodyRange.End.Offset) {
		return source, nil, fmt.Errorf("trigger method extraction: trigger body range does not match source")
	}
	var methods []apexast.ASTNode
	var names []string
	previousEnd := start + 1
	members := body.Children
	// The parser wraps a trigger body in one block with the same range.
	// Unwrap only that structural wrapper, never nested executable blocks.
	if len(members) == 1 && members[0].Kind == "block" &&
		members[0].Range.Start.Offset == start && members[0].Range.End.Offset == end {
		members = members[0].Children
	}
	for _, node := range members {
		if node.Kind != "method_declaration" {
			continue
		}
		methodStart, methodEnd := node.Range.Start.Offset, node.Range.End.Offset
		if methodStart < previousEnd || methodEnd <= methodStart || methodEnd > end-1 {
			return source, nil, fmt.Errorf("trigger method extraction: invalid or overlapping method range")
		}
		identifiers, err := triggerDirectIdentifiers(source, node)
		if err != nil {
			return source, nil, err
		}
		if len(identifiers) != 1 {
			return source, nil, fmt.Errorf("trigger method extraction: ambiguous method identity")
		}
		methods = append(methods, node)
		names = append(names, identifiers[0])
		previousEnd = methodEnd
	}
	if len(methods) == 0 {
		return source, nil, nil
	}
	masked := []byte(source)
	for _, method := range methods {
		for i := method.Range.Start.Offset; i < method.Range.End.Offset; i++ {
			if masked[i] != '\r' && masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	return string(masked), names, nil
}

func triggerDirectIdentifiers(source string, node apexast.ASTNode) ([]string, error) {
	var names []string
	for _, child := range node.Children {
		if child.Kind != "identifier" {
			continue
		}
		start, end := child.Range.Start.Offset, child.Range.End.Offset
		if start < node.Range.Start.Offset || end > node.Range.End.Offset || start < 0 || end <= start || end > len(source) {
			return nil, fmt.Errorf("trigger method extraction: invalid identifier range")
		}
		names = append(names, source[start:end])
	}
	return names, nil
}
