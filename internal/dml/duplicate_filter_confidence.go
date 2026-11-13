package dml

import (
	"strconv"
	"strings"
)

// A custom matching rule's confidence is read off its filter logic. Native
// rows (the OR and three-item rules of the standard-rule fixture) show that
// every AND group and every OR group reports the mean of its children, that a
// leaf is its item's confidence, and that a leaf that did not match still
// counts with the confidence the item's algorithm gave the pair. "(1 AND 2)
// OR 3" with the first name and last name matching and the phone number
// different scores the mean of the name group and the phone item.
type filterConfidenceNode struct {
	op   byte // 'a' AND, 'o' OR, 'l' leaf
	leaf int
	kids []*filterConfidenceNode
}

type filterConfidenceParser struct {
	tokens []formulaToken
	pos    int
	items  int
	bad    bool
}

func (p *filterConfidenceParser) peek() formulaToken {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return formulaToken{typ: formulaTokenEOF}
}

func (p *filterConfidenceParser) isWord(word string) bool {
	t := p.peek()
	return t.typ == formulaTokenIdent && strings.EqualFold(t.text, word)
}

func (p *filterConfidenceParser) parseOr() *filterConfidenceNode {
	first := p.parseAnd()
	if first == nil || !p.isWord("OR") {
		return first
	}
	node := &filterConfidenceNode{op: 'o', kids: []*filterConfidenceNode{first}}
	for p.isWord("OR") {
		p.pos++
		kid := p.parseAnd()
		if kid == nil {
			return nil
		}
		node.kids = append(node.kids, kid)
	}
	return node
}

func (p *filterConfidenceParser) parseAnd() *filterConfidenceNode {
	first := p.parsePrimary()
	if first == nil || !p.isWord("AND") {
		return first
	}
	node := &filterConfidenceNode{op: 'a', kids: []*filterConfidenceNode{first}}
	for p.isWord("AND") {
		p.pos++
		kid := p.parsePrimary()
		if kid == nil {
			return nil
		}
		node.kids = append(node.kids, kid)
	}
	return node
}

func (p *filterConfidenceParser) parsePrimary() *filterConfidenceNode {
	t := p.peek()
	switch {
	case t.typ == formulaTokenNumber:
		index, err := strconv.Atoi(t.text)
		if err != nil || index < 1 || index > p.items {
			p.bad = true
			return nil
		}
		p.pos++
		return &filterConfidenceNode{op: 'l', leaf: index - 1}
	case t.typ == formulaTokenSymbol && t.text == "(":
		p.pos++
		inner := p.parseOr()
		if inner == nil || p.peek().typ != formulaTokenSymbol || p.peek().text != ")" {
			p.bad = true
			return nil
		}
		p.pos++
		return inner
	}
	p.bad = true
	return nil
}

// parseFilterConfidence parses a matching rule's boolean filter over the
// given number of items. ok is false for an expression outside AND, OR and
// parentheses.
func parseFilterConfidence(expression string, items int) (*filterConfidenceNode, bool) {
	var tokens []formulaToken
	for _, token := range tokenizeFormula(expression) {
		if token.typ != formulaTokenEOF {
			tokens = append(tokens, token)
		}
	}
	p := &filterConfidenceParser{tokens: tokens, items: items}
	node := p.parseOr()
	if node == nil || p.bad || p.pos != len(tokens) {
		return nil, false
	}
	return node, true
}

func (n *filterConfidenceNode) evaluate(matches []bool, scores []float64) (bool, float64) {
	if n.op == 'l' {
		return matches[n.leaf], scores[n.leaf]
	}
	all, any, sum := true, false, 0.0
	for _, kid := range n.kids {
		matched, score := kid.evaluate(matches, scores)
		all = all && matched
		any = any || matched
		sum += score
	}
	mean := sum / float64(len(n.kids))
	if n.op == 'a' {
		return all, mean
	}
	return any, mean
}

// filterRuleConfidence evaluates a matching rule's logic with the item
// verdicts and item confidences. An empty expression is every item AND-ed.
func filterRuleConfidence(expression string, matches []bool, scores []float64) (matched bool, confidence float64, ok bool) {
	if len(matches) == 0 {
		return false, 0, false
	}
	if strings.TrimSpace(expression) == "" {
		all, sum := true, 0.0
		for i, m := range matches {
			all = all && m
			sum += scores[i]
		}
		return all, sum / float64(len(matches)), true
	}
	node, ok := parseFilterConfidence(expression, len(matches))
	if !ok {
		return false, 0, false
	}
	matched, confidence = node.evaluate(matches, scores)
	return matched, confidence, true
}
