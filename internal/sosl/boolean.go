package sosl

import (
	"fmt"
	"strings"
	"unicode"
)

// SearchExpression retains Boolean structure without changing the flat Terms
// view used by snippet generation. Nodes are immutable after parsing.
type SearchExpression struct {
	Operator    string
	Term        SearchTerm
	Left, Right *SearchExpression
}

type searchExpressionParser struct {
	tokens []string
	pos    int
	terms  []SearchTerm
}

// parseSearchExpression consumes the already decoded FIND text. Whitespace
// adjacency is conjunction, with the same precedence as explicit AND.
func parseSearchExpression(text string) ([]SearchTerm, *SearchExpression, error) {
	// Tagged escapes distinguish adjacent escaped stars from literal marker
	// runes. Replacement is one-pass so generated tags are never re-encoded.
	text = strings.NewReplacer(escapedSearchWildcardMarker, escapedSearchWildcardMarker+"m", `\*`, escapedSearchWildcardMarker+"w").Replace(text)
	if strings.Contains(text, "\\(") || strings.Contains(text, "\\)") {
		return nil, nil, &UnsupportedFeatureError{Message: "SOSL escaped search grouping"}
	}
	var tokens []string
	start := -1
	quoted := false
	flush := func(i int) {
		if start >= 0 {
			tokens = append(tokens, text[start:i])
			start = -1
		}
	}
	for i, r := range text {
		if quoted {
			if r == '"' {
				tokens = append(tokens, text[start:i+1])
				start, quoted = -1, false
			}
			continue
		}
		if r == '"' {
			flush(i)
			start, quoted = i, true
			continue
		}
		if unicode.IsSpace(r) || r == '(' || r == ')' {
			flush(i)
			if r == '(' || r == ')' {
				tokens = append(tokens, string(r))
			}
		} else if start < 0 {
			start = i
		}
	}
	if quoted {
		return nil, nil, fmt.Errorf("unclosed SOSL search phrase")
	}
	flush(len(text))
	p := searchExpressionParser{tokens: tokens}
	node, err := p.expression(1)
	if err != nil {
		return nil, nil, err
	}
	if p.pos != len(tokens) {
		return nil, nil, fmt.Errorf("unexpected SOSL search token %q", tokens[p.pos])
	}
	return p.terms, node, nil
}

func (p *searchExpressionParser) expression(minPrecedence int) (*SearchExpression, error) {
	left, err := p.primary()
	if err != nil {
		return nil, err
	}
	for p.pos < len(p.tokens) && p.tokens[p.pos] != ")" {
		op := strings.ToUpper(p.tokens[p.pos])
		precedence := 1
		explicit := true
		consume := 1
		switch op {
		case "AND":
			precedence = 2
			if p.pos+1 < len(p.tokens) && strings.EqualFold(p.tokens[p.pos+1], "NOT") {
				op = "AND NOT"
				consume = 2
			}
		case "OR":
		case "NOT":
			return nil, &UnsupportedFeatureError{Message: "SOSL boolean search operator NOT"}
		default:
			op, explicit, precedence = "AND", false, 2
		}
		if precedence < minPrecedence {
			break
		}
		if explicit {
			p.pos += consume
		}
		nextPrecedence := precedence + 1
		if precedence == 2 {
			// AND and AND NOT associate right-to-left in FIND expressions.
			nextPrecedence = precedence
		}
		right, err := p.expression(nextPrecedence)
		if err != nil {
			return nil, err
		}
		left = &SearchExpression{Operator: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *searchExpressionParser) primary() (*SearchExpression, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("expected SOSL search operand")
	}
	item := p.tokens[p.pos]
	p.pos++
	if item == "(" {
		node, err := p.expression(1)
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos] != ")" {
			return nil, fmt.Errorf("unclosed SOSL search group")
		}
		p.pos++
		return node, nil
	}
	switch strings.ToUpper(item) {
	case ")", "AND", "OR":
		return nil, fmt.Errorf("expected SOSL search operand, got %q", item)
	case "NOT":
		return nil, &UnsupportedFeatureError{Message: "SOSL boolean search operator NOT"}
	}
	prefix := strings.HasSuffix(item, "*")
	literal := item
	if prefix {
		literal = strings.TrimSuffix(literal, "*")
	}
	term := SearchTerm{Text: decodeSearchWildcardMarkers(literal), Prefix: prefix}
	if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
		term = SearchTerm{Text: decodeSearchWildcardMarkers(item[1 : len(item)-1])}
	}
	// Salesforce accepts a bare leading wildcard (`*`) as a search operand.
	// Keep the empty text with Prefix=true so runtime matching treats it as
	// the broad wildcard while still rejecting genuinely empty operands.
	if term.Text == "" && !term.Prefix {
		return nil, fmt.Errorf("empty SOSL search operand")
	}
	p.terms = append(p.terms, term)
	return &SearchExpression{Operator: "TERM", Term: term}, nil
}

func decodeSearchWildcardMarkers(text string) string {
	return strings.NewReplacer(escapedSearchWildcardMarker+"w", "*", escapedSearchWildcardMarker+"m", escapedSearchWildcardMarker).Replace(text)
}
