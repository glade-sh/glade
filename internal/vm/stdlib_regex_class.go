package vm

import (
	"fmt"
	"strings"
)

type javaClassExpr interface {
	javaClassPredicate() string
}

type javaClassSimple struct {
	body string
}

type javaClassIntersection struct {
	operands []javaClassExpr
}

type javaClassUnion struct {
	operands []javaClassExpr
}

type javaClassNegation struct {
	inner javaClassExpr
}

func rewriteJavaClassAlgebraForRegexp2(source string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(source); {
		if source[i] != '[' || isEscapedRegexByte(source, i) {
			out.WriteByte(source[i])
			i++
			continue
		}
		end := javaRegexCharClassEnd(source, i)
		if end < 0 {
			out.WriteByte(source[i])
			i++
			continue
		}
		body := source[i+1 : end]
		if !javaClassNeedsLowering(body) {
			out.WriteString(source[i : end+1])
			i = end + 1
			continue
		}
		expr, err := parseJavaClassExpr(body)
		if err != nil {
			return "", err
		}
		out.WriteString(javaClassAtom(expr))
		i = end + 1
	}
	return out.String(), nil
}

func parseJavaClassExpr(body string) (javaClassExpr, error) {
	if body == "" {
		return nil, fmt.Errorf("Java regex character-class intersections")
	}
	if strings.HasPrefix(body, "^") {
		inner, err := parseJavaClassExpr(body[1:])
		if err != nil {
			return nil, err
		}
		return javaClassNegation{inner: inner}, nil
	}
	parts, ok := splitJavaClassIntersectionOperands(body)
	if !ok || len(parts) == 0 {
		return nil, fmt.Errorf("Java regex character-class intersections")
	}
	if len(parts) == 1 {
		return parseJavaClassUnion(body)
	}
	operands := make([]javaClassExpr, 0, len(parts))
	for _, part := range parts {
		expr, err := javaClassIntersectionOperand(part)
		if err != nil {
			return nil, err
		}
		operands = append(operands, expr)
	}
	return javaClassIntersection{operands: operands}, nil
}

func javaClassAtom(expr javaClassExpr) string {
	return "(?:" + expr.javaClassPredicate() + `[\s\S]` + ")"
}

func (c javaClassSimple) javaClassPredicate() string {
	return "(?=[" + c.body + "])"
}

func (c javaClassIntersection) javaClassPredicate() string {
	var out strings.Builder
	for _, operand := range c.operands {
		out.WriteString(operand.javaClassPredicate())
	}
	return out.String()
}

func (c javaClassUnion) javaClassPredicate() string {
	var out strings.Builder
	out.WriteString("(?:")
	for i, operand := range c.operands {
		if i > 0 {
			out.WriteByte('|')
		}
		out.WriteString(operand.javaClassPredicate())
	}
	out.WriteByte(')')
	return out.String()
}

func (c javaClassNegation) javaClassPredicate() string {
	return "(?!" + javaClassAtom(c.inner) + ")"
}

func javaClassNeedsLowering(body string) bool {
	if strings.Contains(body, "&&") {
		return true
	}
	for i := 0; i < len(body); i++ {
		if isEscapedRegexByte(body, i) {
			continue
		}
		if body[i] == '[' {
			return true
		}
	}
	return false
}

func parseJavaClassUnion(body string) (javaClassExpr, error) {
	var operands []javaClassExpr
	var literal strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) {
			literal.WriteByte(body[i])
			i++
			literal.WriteByte(body[i])
			continue
		}
		if body[i] != '[' {
			literal.WriteByte(body[i])
			continue
		}
		if literal.Len() > 0 {
			operands = append(operands, javaClassSimple{body: literal.String()})
			literal.Reset()
		}
		end := javaClassNestedEnd(body, i)
		if end < 0 {
			return nil, fmt.Errorf("Java regex character-class intersections")
		}
		nested, err := parseJavaClassExpr(body[i+1 : end])
		if err != nil {
			return nil, err
		}
		operands = append(operands, nested)
		i = end
	}
	if literal.Len() > 0 {
		operands = append(operands, javaClassSimple{body: literal.String()})
	}
	if len(operands) == 0 {
		return javaClassSimple{body: body}, nil
	}
	if len(operands) == 1 {
		return operands[0], nil
	}
	return javaClassUnion{operands: operands}, nil
}

func splitJavaClassIntersectionOperands(body string) ([]string, bool) {
	var parts []string
	start := 0
	nested := 0
	for i := 0; i < len(body); i++ {
		if isEscapedRegexByte(body, i) {
			continue
		}
		switch body[i] {
		case '[':
			nested++
		case ']':
			if nested == 0 {
				return nil, false
			}
			nested--
		case '&':
			if nested == 0 && i+1 < len(body) && body[i+1] == '&' {
				parts = append(parts, body[start:i])
				i++
				start = i + 1
			}
		}
	}
	if nested != 0 {
		return nil, false
	}
	parts = append(parts, body[start:])
	return parts, true
}

func javaClassIntersectionOperand(part string) (javaClassExpr, error) {
	if part == "" {
		return nil, fmt.Errorf("Java regex character-class intersections")
	}
	if strings.HasPrefix(part, "[") {
		if !strings.HasSuffix(part, "]") {
			return nil, fmt.Errorf("Java regex character-class intersections")
		}
		return parseJavaClassExpr(part[1 : len(part)-1])
	}
	return parseJavaClassExpr(part)
}

func javaRegexCharClassEnd(source string, start int) int {
	nested := 0
	for i := start + 1; i < len(source); i++ {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case '[':
			nested++
		case ']':
			if nested > 0 {
				nested--
				continue
			}
			return i
		}
	}
	return -1
}

func javaClassNestedEnd(body string, start int) int {
	nested := 0
	for i := start + 1; i < len(body); i++ {
		if isEscapedRegexByte(body, i) {
			continue
		}
		switch body[i] {
		case '[':
			nested++
		case ']':
			if nested > 0 {
				nested--
				continue
			}
			return i
		}
	}
	return -1
}
