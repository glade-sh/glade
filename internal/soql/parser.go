package soql

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

var (
	soqlSeparatorBytes     = byteSet(",=!*()<>")
	soqlTokenBoundaryBytes = byteSet(" \n\t\r,=!*()<>")
	soqlLiteralStopBytes   = byteSet(",)(")
)

func byteSet(chars string) [256]bool {
	var out [256]bool
	for i := 0; i < len(chars); i++ {
		out[chars[i]] = true
	}
	return out
}

// ContractError identifies a parsed query rejected by shared semantic
// validation. Callers can preserve contract diagnostics without parsing text.
type ContractError struct{ Err error }

func (e *ContractError) Error() string { return e.Err.Error() }
func (e *ContractError) Unwrap() error { return e.Err }

// QueryError retains the parser diagnostic and the externally observable
// QueryException message. Runtime callers need not interpret diagnostic text.
type QueryError struct {
	Message string
	// DateTimeLexerMessage is the source-positioned diagnostic exposed by
	// versioned Database.countQuery callers for malformed datetime tokens.
	DateTimeLexerMessage string
	// Uncatchable distinguishes native query-contract failures from QueryException.
	Uncatchable bool
	// CompileSyntax distinguishes grammar recovery from semantic rejections.
	CompileSyntax bool
	// Query handles expose a captured diagnostic for this shape. Other callers
	// retain Error's parser text unless they explicitly handle this marker.
	NonGroupedAggregateLimit bool
	Err                      error
}

func (e *QueryError) Error() string { return e.Err.Error() }
func (e *QueryError) Unwrap() error { return e.Err }

func queryError(message, detail string) error {
	return &QueryError{Message: message, Err: fmt.Errorf("soql: %s", detail)}
}

func unexpectedQueryError(message, detail string) error {
	return &QueryError{Message: message, Uncatchable: true, Err: fmt.Errorf("soql: %s", detail)}
}

func querySyntaxError(message, detail string) error {
	return &QueryError{Message: message, CompileSyntax: true, Err: fmt.Errorf("soql: %s", detail)}
}

func unexpectedToken(text, detail string) error {
	if text == "" {
		text = "<EOF>"
	}
	return querySyntaxError("unexpected token: '"+text+"'", detail)
}

type token struct {
	text       string
	bindOffset int
}

func lex(input string) ([]token, error) {
	var out []token
	for i := 0; i < len(input); {
		switch {
		case input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r':
			i++
		case i+1 < len(input) && input[i] == '/' && input[i+1] == '/':
			i += 2
			for i < len(input) && input[i] != '\n' && input[i] != '\r' {
				i++
			}
		case i+1 < len(input) && input[i] == '/' && input[i+1] == '*':
			i += 2
			for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
				i++
			}
			if i+1 >= len(input) {
				return nil, fmt.Errorf("soql: unterminated block comment")
			}
			i += 2
		case input[i] == '\'':
			start := i
			i++
			escaped := false
			for i < len(input) {
				if escaped {
					escaped = false
					i++
					continue
				}
				if input[i] == '\\' {
					escaped = true
					i++
					continue
				}
				if input[i] == '\'' {
					if i+1 < len(input) && input[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					out = append(out, token{text: input[start:i]})
					goto next
				}
				i++
			}
			return nil, unexpectedToken("", "unterminated string literal")
		case input[i] == ':':
			start := i
			i++
			for i < len(input) && (input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r') {
				i++
			}
			bindStart := i
			if end, ok := scanSOQLCastBind(input, bindStart); ok {
				out = append(out, token{bindOffset: start, text: ":" + strings.TrimSpace(input[bindStart:end])})
				i = end
				continue
			}
			if end, ok := scanSOQLStringMethodBind(input, bindStart); ok {
				end = scanSOQLBindAdditiveTail(input, end)
				out = append(out, token{bindOffset: start, text: ":" + input[bindStart:end]})
				i = end
				continue
			}
			if bindStart == len(input) || !soqlBindIdentifierStart(input[bindStart]) {
				// A numbered date literal accepts optional whitespace after its
				// colon (for example, N_MONTHS_AGO : 1). Only absorb whitespace
				// into an Apex bind when the following token starts an identifier.
				out = append(out, token{bindOffset: start, text: ":"})
				i = start + 1
				continue
			}
			if end, ok := scanSOQLBindCollectionConstructor(input, bindStart); ok {
				out = append(out, token{bindOffset: start, text: ":" + strings.TrimSpace(input[bindStart:end])})
				i = end
				continue
			}
			depth := 0
			for i < len(input) {
				if input[i] == '(' {
					depth++
					i++
					continue
				}
				if input[i] == ')' {
					if depth == 0 {
						break
					}
					depth--
					i++
					continue
				}
				if depth == 0 && (soqlTokenBoundaryBytes[input[i]] || input[i] == ':') {
					break
				}
				i++
			}
			i = scanSOQLBindDottedTail(input, i)
			i = scanSOQLBindAdditiveTail(input, i)
			if bindStart == i {
				out = append(out, token{bindOffset: start, text: input[start:i]})
			} else {
				out = append(out, token{bindOffset: start, text: ":" + strings.TrimSpace(input[bindStart:i])})
			}
		case soqlSeparatorBytes[input[i]]:
			if i+1 < len(input) && input[i:i+2] == "!=" {
				out = append(out, token{text: "!="})
				i += 2
			} else if i+1 < len(input) && input[i:i+2] == "<=" {
				out = append(out, token{text: "<="})
				i += 2
			} else if i+1 < len(input) && input[i:i+2] == ">=" {
				out = append(out, token{text: ">="})
				i += 2
			} else {
				out = append(out, token{text: input[i : i+1]})
				i++
			}
		default:
			start := i
			for i < len(input) && !soqlTokenBoundaryBytes[input[i]] && (input[i] != ':' || !soqlBindIdentifierStart(input[start])) {
				i++
			}
			if looksLikeISODateTime(input[start:i]) {
				// A comma in the fraction belongs to the malformed datetime,
				// rather than separating two otherwise valid list operands.
				if i-start == 19 && i+1 < len(input) && input[i] == ',' && isASCIIDigit(input[i+1]) {
					i++
					for i < len(input) && !soqlTokenBoundaryBytes[input[i]] {
						i++
					}
				}
				if err := validateISODateTimeToken(input, start, input[start:i]); err != nil {
					return nil, err
				}
			}
			out = append(out, token{text: input[start:i]})
		}
	next:
	}
	out = append(out, token{text: ""})
	return out, nil
}

// Retain an Apex cast and its operand as one bind. The Apex compiler still
// validates the type and expression; SOQL only needs the expression boundary.
func scanSOQLCastBind(input string, start int) (int, bool) {
	if start >= len(input) || input[start] != '(' {
		return 0, false
	}
	end := strings.IndexByte(input[start+1:], ')')
	if end < 0 {
		return 0, false
	}
	end += start + 1
	typeName := strings.TrimSpace(input[start+1 : end])
	if typeName == "" || !soqlBindIdentifierStart(typeName[0]) {
		return 0, false
	}
	for _, ch := range typeName {
		if ch > 127 || !(soqlBindIdentifierPart(byte(ch)) || strings.ContainsRune(".<>[], \t\n\r", ch)) {
			return 0, false
		}
	}
	operand := end + 1
	for operand < len(input) && strings.ContainsRune(" \t\n\r", rune(input[operand])) {
		operand++
	}
	if nested, ok := scanSOQLCastBind(input, operand); ok {
		return scanSOQLBindAdditiveTail(input, nested), true
	}
	end, ok := scanSOQLBindAdditiveOperand(input, operand)
	if !ok {
		return 0, false
	}
	return scanSOQLBindAdditiveTail(input, end), true
}

// scanSOQLStringMethodBind retains documented Apex binds such as
// :'XXXX'.substring(0, 3) as one SOQL value token. The expression itself is
// parsed by Apex before query execution, so the SOQL lexer must not split the
// literal from its member call.
func scanSOQLStringMethodBind(input string, start int) (int, bool) {
	if start >= len(input) || input[start] != '\'' {
		return 0, false
	}
	i := start + 1
	escaped := false
	for i < len(input) {
		if escaped {
			escaped = false
			i++
			continue
		}
		if input[i] == '\\' {
			escaped = true
			i++
			continue
		}
		if input[i] != '\'' {
			i++
			continue
		}
		if i+1 < len(input) && input[i+1] == '\'' {
			i += 2
			continue
		}
		i++
		break
	}
	if i > len(input) || i == len(input) && input[i-1] != '\'' {
		return 0, false
	}
	if i == len(input) || input[i] != '.' {
		return i, true
	}
	for i < len(input) && input[i] == '.' {
		i++
		memberStart := i
		for i < len(input) && soqlBindIdentifierPart(input[i]) {
			i++
		}
		if memberStart == i {
			return 0, false
		}
		if i == len(input) || input[i] != '(' {
			continue
		}
		depth := 0
		for i < len(input) {
			switch input[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					i++
					goto nextMember
				}
			}
			i++
		}
		return 0, false
	nextMember:
	}
	return i, true
}

func scanSOQLBindAdditiveTail(input string, start int) int {
	end := start
	for {
		operator := end
		for operator < len(input) && (input[operator] == ' ' || input[operator] == '\n' || input[operator] == '\t' || input[operator] == '\r') {
			operator++
		}
		if operator == len(input) || (input[operator] != '+' && input[operator] != '-') {
			return end
		}
		operand := operator + 1
		for operand < len(input) && (input[operand] == ' ' || input[operand] == '\n' || input[operand] == '\t' || input[operand] == '\r') {
			operand++
		}
		operandEnd, ok := scanSOQLBindAdditiveOperand(input, operand)
		if !ok {
			return end
		}
		end = operandEnd
	}
}

// scanSOQLBindDottedTail keeps compiler-spaced Apex member binds such as
// ": account . Id" together. A whitespace-delimited clause is not part of the
// bind unless a dot and a following identifier are both present.
func scanSOQLBindDottedTail(input string, start int) int {
	end := start
	for {
		dot := end
		for dot < len(input) && (input[dot] == ' ' || input[dot] == '\n' || input[dot] == '\t' || input[dot] == '\r') {
			dot++
		}
		if dot < len(input) && input[dot] == '.' {
			dot++
		} else if end == 0 || input[end-1] != '.' {
			return end
		}
		for dot < len(input) && (input[dot] == ' ' || input[dot] == '\n' || input[dot] == '\t' || input[dot] == '\r') {
			dot++
		}
		if dot == len(input) || !soqlBindIdentifierStart(input[dot]) {
			return end
		}
		dot++
		for dot < len(input) && soqlBindIdentifierPart(input[dot]) {
			dot++
		}
		end = dot
	}
}

func scanSOQLBindAdditiveOperand(input string, start int) (int, bool) {
	if start >= len(input) {
		return 0, false
	}
	if input[start] == '\'' {
		return scanSOQLStringMethodBind(input, start)
	}
	if !soqlBindIdentifierStart(input[start]) && (input[start] < '0' || input[start] > '9') {
		return 0, false
	}
	depth := 0
	for i := start; i < len(input); i++ {
		switch input[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i, i > start
			}
			depth--
		default:
			if depth == 0 && (soqlTokenBoundaryBytes[input[i]] || input[i] == ':') {
				return i, i > start
			}
		}
	}
	return len(input), true
}

func soqlBindIdentifierPart(value byte) bool {
	return soqlBindIdentifierStart(value) || value >= '0' && value <= '9'
}

func soqlBindIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func scanSOQLBindCollectionConstructor(input string, start int) (int, bool) {
	if start+3 >= len(input) || !strings.EqualFold(input[start:start+3], "new") || input[start+3] != ' ' {
		return 0, false
	}
	// Collection literals (new Set<Id>{...}) and constructor expressions
	// (new Map<Id, Opportunity>(input).keySet()) are valid bind expressions.
	openBrace := strings.IndexByte(input[start+4:], '{')
	openParen := strings.IndexByte(input[start+4:], '(')
	if openBrace < 0 || (openParen >= 0 && openParen < openBrace) {
		if openParen < 0 {
			return 0, false
		}
		openParen += start + 4
		depth := 0
		end := openParen
		for ; end < len(input); end++ {
			switch input[end] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end++
					goto members
				}
			}
		}
		return 0, false
	members:
		for end < len(input) && input[end] == '.' {
			end++
			for end < len(input) && soqlBindIdentifierPart(input[end]) {
				end++
			}
			if end >= len(input) || input[end] != '(' {
				return 0, false
			}
			depth = 0
			for ; end < len(input); end++ {
				if input[end] == '(' {
					depth++
				} else if input[end] == ')' {
					depth--
					if depth == 0 {
						end++
						break
					}
				}
			}
		}
		return end, true
	}
	openBrace += start + 4
	depth := 0
	for index := openBrace; index < len(input); index++ {
		switch input[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return 0, false
}

// BindColumn identifies a predicate bind occurrence before literal lowering.
// Offsets distinguish the same variable used against text and Id columns.
type BindColumn struct {
	Object, Field string
}

func BindColumns(input string) (map[int]BindColumn, error) {
	tokens, err := lex(input)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens, now: time.Now(), fiscalYearStartMonth: 1,
		dateLiteralTimeZoneID: "UTC", bindColumns: make(map[int]BindColumn)}
	if _, err := p.parseQuery(); err != nil {
		return nil, err
	}
	if p.peek().text != "" {
		return nil, unexpectedToken(p.peek().text, "unexpected token after query")
	}
	return p.bindColumns, nil
}

type parser struct {
	bindColumns           map[int]BindColumn
	bindObject            string
	tokens                []token
	pos                   int
	now                   time.Time
	fiscalYearStartMonth  int
	dateLiteralTimeZoneID string
}

func (p *parser) parseQuery() (Query, error) {
	if !p.matchWord("SELECT") {
		return Query{}, unexpectedToken(p.peek().text, "expected SELECT")
	}
	fields, childQueries, typeofs, err := p.parseFields()
	if err != nil {
		return Query{}, err
	}
	if len(fields) == 0 && len(childQueries) == 0 && len(typeofs) == 0 {
		return Query{}, unexpectedToken("SELECT FROM", "SELECT requires at least one field")
	}
	if !p.matchWord("FROM") {
		// Preserve the dynamic diagnostic while tagging compile-time recovery.
		return Query{}, querySyntaxError("", "expected FROM")
	}
	object, err := p.parseName()
	if err != nil {
		return Query{}, err
	}
	savedObject := p.bindObject
	p.bindObject = object
	defer func() { p.bindObject = savedObject }()
	relationshipAliases, err := p.parseRootObjectAlias()
	if err != nil {
		return Query{}, err
	}
	moreAliases, err := p.parseRelationshipAliases(object, relationshipAliases)
	if err != nil {
		return Query{}, err
	}
	for alias, path := range moreAliases {
		if relationshipAliases == nil {
			relationshipAliases = make(map[string]string)
		}
		relationshipAliases[alias] = path
	}
	if len(relationshipAliases) != 0 {
		fields = rewriteRelationshipAliasNames(fields, relationshipAliases)
	}
	aggregates, err := aggregateSpecs(fields)
	if err != nil {
		return Query{}, err
	}
	q := Query{Fields: fields, ChildQueries: childQueries, Typeofs: typeofs, Object: object, Count: len(fields) == 1 && strings.EqualFold(fields[0], "COUNT()"), Aggregates: aggregates}
	lastClauseRank := 0
	seenClauses := make(map[string]bool)
	checkClauseOrder := func(name string, rank int) error {
		token := strings.Fields(name)[0]
		if name != "FOR" && seenClauses[name] {
			return unexpectedToken(token, fmt.Sprintf("duplicate %s clause", name))
		}
		outOfOrder := rank < lastClauseRank
		if name == "FOR" && seenClauses["ALL ROWS"] {
			// Keep the established ALL ROWS FOR UPDATE/VIEW form valid even
			// though its lock/view clause follows ALL ROWS in the parser input.
			outOfOrder = false
		}
		if outOfOrder {
			return unexpectedToken(token, fmt.Sprintf("%s clause is out of order", name))
		}
		if name != "FOR" {
			seenClauses[name] = true
		}
		lastClauseRank = rank
		return nil
	}
	for p.peek().text != "" && p.peek().text != ")" {
		switch {
		case p.matchWord("WHERE"):
			if err := checkClauseOrder("WHERE", 2); err != nil {
				return Query{}, err
			}
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			if len(relationshipAliases) != 0 {
				rewriteRelationshipAliasCondition(&condition, relationshipAliases)
			}
			q.Where = &condition
		case p.matchWord("GROUP"):
			if err := checkClauseOrder("GROUP BY", 4); err != nil {
				return Query{}, err
			}
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after GROUP")
			}
			groupMode := ""
			if p.matchWord("ROLLUP") || p.matchWord("CUBE") {
				groupMode = strings.ToUpper(p.tokens[p.pos-1].text)
				if !p.match("(") {
					return Query{}, p.errorf("expected ( after GROUP BY %s", groupMode)
				}
				if p.peek().text == ")" {
					return Query{}, unexpectedToken(")", "empty grouping expression")
				}
			}
			groupBy, err := p.parseNameList()
			if err != nil {
				return Query{}, err
			}
			if len(relationshipAliases) != 0 {
				groupBy = rewriteRelationshipAliasNames(groupBy, relationshipAliases)
			}
			if groupMode != "" && !p.match(")") {
				return Query{}, p.errorf("expected ) after GROUP BY %s", groupMode)
			}
			q.GroupBy = groupBy
			q.GroupMode = groupMode
		case p.matchWord("HAVING"):
			if err := checkClauseOrder("HAVING", 5); err != nil {
				return Query{}, err
			}
			if len(q.GroupBy) == 0 {
				return Query{}, unexpectedToken("HAVING", "HAVING requires GROUP BY")
			}
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			condition = rewriteHavingAggregates(condition, &q)
			q.Having = &condition
		case p.matchWord("ORDER"):
			if err := checkClauseOrder("ORDER BY", 6); err != nil {
				return Query{}, err
			}
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after ORDER")
			}
			order, err := p.parseOrderList()
			if err != nil {
				return Query{}, err
			}
			if len(relationshipAliases) != 0 {
				for i := range order {
					order[i].Field = rewriteRelationshipAliasName(order[i].Field, relationshipAliases)
				}
			}
			if len(order) == 0 {
				return Query{}, p.errorf("expected ORDER BY field")
			}
			q.Order = order
			q.OrderBy = order[0].Field
			q.OrderDesc = order[0].Desc
		case p.matchWord("LIMIT"):
			if err := checkClauseOrder("LIMIT", 7); err != nil {
				return Query{}, err
			}
			limit, bind, err := p.parseIntOrBind("LIMIT")
			if err != nil {
				return Query{}, err
			}
			q.Limit = limit
			q.LimitBind = bind
			q.HasLimit = true
		case p.matchWord("OFFSET"):
			if err := checkClauseOrder("OFFSET", 8); err != nil {
				return Query{}, err
			}
			offset, bind, err := p.parseIntOrBind("OFFSET")
			if err != nil {
				if queryErr, ok := err.(*QueryError); ok && queryErr.Message == "SOQL offset must be a non-negative value" {
					// R219/Q004-Q008: an overall aggregate's OFFSET contract
					// precedes the negative-number check, but not syntax errors.
					if aggregateErr := validateAggregateOffset(q); aggregateErr != nil {
						return Query{}, aggregateErr
					}
				}
				return Query{}, err
			}
			q.Offset = offset
			q.OffsetBind = bind
			q.HasOffset = true
		case p.matchWord("FOR"):
			if err := checkClauseOrder("FOR", 9); err != nil {
				return Query{}, err
			}
			if q.ForUpdate || q.ForView || q.ForReference {
				token := "FOR"
				if !q.ForUpdate && !strings.EqualFold(p.peek().text, "UPDATE") {
					token = p.peek().text
				}
				return Query{}, unexpectedToken(token, "duplicate FOR clause")
			}
			switch {
			case p.matchWord("UPDATE"):
				q.ForUpdate = true
			case p.matchWord("VIEW"):
				q.ForView = true
			case p.matchWord("REFERENCE"):
				q.ForReference = true
			default:
				return Query{}, p.errorf("expected UPDATE, VIEW, or REFERENCE after FOR")
			}
		case p.matchWord("ALL"):
			if err := checkClauseOrder("ALL ROWS", 10); err != nil {
				return Query{}, err
			}
			if !p.matchWord("ROWS") {
				return Query{}, p.errorf("expected ROWS after ALL")
			}
			q.AllRows = true
		case p.matchWord("USING"):
			if err := checkClauseOrder("USING SCOPE", 1); err != nil {
				return Query{}, err
			}
			if !p.matchWord("SCOPE") {
				return Query{}, p.errorf("expected SCOPE after USING")
			}
			scope, err := p.parseName()
			if err != nil {
				return Query{}, err
			}
			q.UsingScope = scope
		case p.matchWord("WITH"):
			if err := checkClauseOrder("WITH", 3); err != nil {
				return Query{}, err
			}
			mode, err := p.parseSecurityMode()
			if err != nil {
				return Query{}, err
			}
			q.SecurityMode = mode
		default:
			return Query{}, &QueryError{Message: "unexpected token: '" + p.peek().text + "'", Err: unsupportedSOQLErrorf("unsupported SOQL token %q", p.peek().text)}
		}
	}
	if len(q.GroupBy) == 0 {
		for _, field := range q.Fields {
			if _, _, aliased := splitSelectFieldAlias(field); aliased {
				return Query{}, queryError("only aggregate expressions use field aliasing", "only aggregate expressions use field aliasing")
			}
		}
	}
	if err := validateAggregateQuery(q); err != nil {
		return Query{}, &ContractError{Err: err}
	}
	if err := validateDateFunctionGrouping(q, storage.OrgState{}, nil); err != nil {
		return Query{}, &ContractError{Err: err}
	}
	// Selected aggregate expressions order by the same stored result as exprN.
	// Unselected expressions remain subject to the existing reference validator.
	for i := range q.Order {
		for j, aggregate := range q.Aggregates {
			if strings.EqualFold(q.Order[i].Field, aggregateExpression(aggregate)) {
				q.Order[i].Field = fmt.Sprintf("expr%d", j)
				q.Order[i].RewrittenAggregate = true
				break
			}
		}
	}
	if len(q.Order) > 0 {
		q.OrderBy = q.Order[0].Field
	}
	if err := validateSemiAntiQuery(q); err != nil {
		return Query{}, &ContractError{Err: err}
	}
	return q, nil
}

func validateSemiAntiQuery(query Query) error {
	joins, hasOr, err := semiAntiConditions(query.Where)
	if err != nil {
		return err
	}
	if len(joins) > 2 {
		return fmt.Errorf("soql: a WHERE clause can contain at most two semi-join or anti-join subqueries")
	}
	if len(joins) > 0 && hasOr {
		return unexpectedToken("OR", "semi-join and anti-join subqueries cannot be combined with OR")
	}
	for _, join := range joins {
		if err := validateSemiAntiSubquery(query, *join.Subquery); err != nil {
			return err
		}
	}
	if query.Having != nil {
		if err := validateHavingSubqueries(query.Having); err != nil {
			return err
		}
	}
	for _, child := range query.ChildQueries {
		if queryHasSemiAntiJoin(child.Query) {
			return fmt.Errorf("soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause")
		}
	}
	return nil
}

func semiAntiConditions(condition *Condition) ([]Condition, bool, error) {
	var joins []Condition
	hasOr := false
	var walk func(*Condition, bool) error
	walk = func(current *Condition, negated bool) error {
		if current == nil {
			return nil
		}
		negated = negated || current.Not
		if current.Subquery != nil {
			if !strings.EqualFold(current.Op, "IN") && !strings.EqualFold(current.Op, "NOT IN") {
				return fmt.Errorf("soql: semi-join subqueries require IN or NOT IN")
			}
			if negated {
				return fmt.Errorf("soql: use NOT IN for an anti-join; a semi-join cannot be negated with NOT")
			}
			joins = append(joins, *current)
		}
		if len(current.Or) > 0 {
			hasOr = true
		}
		for i := range current.And {
			if err := walk(&current.And[i], negated); err != nil {
				return err
			}
		}
		for i := range current.Or {
			if err := walk(&current.Or[i], negated); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(condition, false); err != nil {
		return nil, false, err
	}
	return joins, hasOr, nil
}

func validateSemiAntiSubquery(main, subquery Query) error {
	if len(subquery.Fields) != 1 || subquery.Count || len(subquery.Aggregates) > 0 {
		return fmt.Errorf("soql: semi-join subquery must select exactly one field")
	}
	if strings.EqualFold(main.Object, subquery.Object) {
		return queryError("The inner and outer selects should not be on the same object type", "semi-join and anti-join subqueries cannot query the same object as the outer query")
	}
	switch strings.ToLower(subquery.Object) {
	case "activityhistory", "attachment", "attachments", "event", "note", "openactivity", "tag", "tags", "task", "tasks":
		return fmt.Errorf("soql: %s is not supported in semi-join or anti-join subqueries", subquery.Object)
	}
	field := strings.TrimSpace(subquery.Fields[0])
	if strings.ContainsAny(field, ".()") {
		return fmt.Errorf("soql: semi-join subquery must select a single ID or reference field")
	}
	if queryHasSemiAntiJoin(subquery) {
		return fmt.Errorf("soql: semi-join and anti-join subqueries cannot be nested")
	}
	if len(subquery.Order) > 0 || subquery.OrderBy != "" || subquery.HasLimit || subquery.Limit != 0 || subquery.LimitBind != "" || subquery.ForUpdate {
		clause := "FOR"
		if subquery.HasLimit || subquery.Limit != 0 || subquery.LimitBind != "" {
			clause = "LIMIT"
		}
		if len(subquery.Order) > 0 || subquery.OrderBy != "" {
			clause = "ORDER"
		}
		return querySyntaxError("missing value at '"+clause+"'", "semi-join subqueries do not support ORDER BY, LIMIT, or FOR UPDATE")
	}
	return nil
}

func queryHasSemiAntiJoin(query Query) bool {
	if conditionHasSemiAntiJoin(query.Where) || conditionHasSemiAntiJoin(query.Having) {
		return true
	}
	for _, child := range query.ChildQueries {
		if queryHasSemiAntiJoin(child.Query) {
			return true
		}
	}
	return false
}

func conditionHasSemiAntiJoin(condition *Condition) bool {
	if condition == nil {
		return false
	}
	if condition.Subquery != nil {
		return true
	}
	for i := range condition.And {
		if conditionHasSemiAntiJoin(&condition.And[i]) {
			return true
		}
	}
	for i := range condition.Or {
		if conditionHasSemiAntiJoin(&condition.Or[i]) {
			return true
		}
	}
	return false
}

func (p *parser) parseRootObjectAlias() (map[string]string, error) {
	if isSOQLClauseStart(p.peek().text) || p.peek().text == "" || p.peek().text == ")" || p.peek().text == "," {
		return nil, nil
	}
	alias, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return map[string]string{alias: ""}, nil
}
func (p *parser) parseRelationshipAliases(rootObject string, rootAliases map[string]string) (map[string]string, error) {
	aliases := map[string]string(nil)
	for p.match(",") {
		path, err := p.parseName()
		if err != nil {
			return nil, err
		}
		path = rewriteRelationshipAliasName(path, rootAliases)
		path = trimRelationshipAliasRoot(path, rootObject)
		if isSOQLClauseStart(p.peek().text) || p.peek().text == "" || p.peek().text == ")" || p.peek().text == "," {
			continue
		}
		alias, err := p.parseName()
		if err != nil {
			return nil, err
		}
		if aliases == nil {
			aliases = make(map[string]string)
		}
		aliases[alias] = path
	}
	return aliases, nil
}
func trimRelationshipAliasRoot(path, rootObject string) string {
	prefix := strings.TrimSpace(rootObject) + "."
	if len(path) > len(prefix) && strings.EqualFold(path[:len(prefix)], prefix) {
		return path[len(prefix):]
	}
	return path
}
func isSOQLClauseStart(text string) bool {
	switch strings.ToUpper(text) {
	case "WHERE", "GROUP", "HAVING", "ORDER", "LIMIT", "OFFSET", "FOR", "ALL", "USING", "WITH":
		return true
	default:
		return false
	}
}
func rewriteRelationshipAliasNames(names []string, aliases map[string]string) []string {
	if len(names) == 0 || len(aliases) == 0 {
		return names
	}
	out := append([]string(nil), names...)
	for i, name := range out {
		out[i] = rewriteRelationshipAliasName(name, aliases)
	}
	return out
}
func rewriteRelationshipAliasName(name string, aliases map[string]string) string {
	for alias, path := range aliases {
		if strings.EqualFold(name, alias) {
			return path
		}
		prefix := alias + "."
		if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
			if path == "" {
				return name[len(prefix):]
			}
			return path + name[len(alias):]
		}
	}
	return name
}
func rewriteRelationshipAliasCondition(condition *Condition, aliases map[string]string) {
	if condition == nil || len(aliases) == 0 {
		return
	}
	condition.Field = rewriteRelationshipAliasName(condition.Field, aliases)
	for i := range condition.And {
		rewriteRelationshipAliasCondition(&condition.And[i], aliases)
	}
	for i := range condition.Or {
		rewriteRelationshipAliasCondition(&condition.Or[i], aliases)
	}
	if condition.Subquery != nil {
		condition.Subquery.Fields = rewriteRelationshipAliasNames(condition.Subquery.Fields, aliases)
	}
}
func (p *parser) parseFields() ([]string, []ChildQuery, []TypeofSpec, error) {
	var fields []string
	var childQueries []ChildQuery
	var typeofs []TypeofSpec
	for {
		if p.match(",") {
			return nil, nil, nil, unexpectedToken(",", "expected selected field")
		}
		if p.peek().text == "" || p.peek().text == ")" || strings.EqualFold(p.peek().text, "FROM") {
			if p.pos > 0 && p.tokens[p.pos-1].text == "," {
				return nil, nil, nil, unexpectedToken(p.peek().text, "expected selected field after comma")
			}
			return fields, childQueries, typeofs, nil
		}
		if p.peek().text == "*" {
			return nil, nil, nil, unexpectedToken("SELECT *", "unsupported wildcard projection")
		}
		if p.match("(") {
			query, err := p.parseQuery()
			if err != nil {
				return nil, nil, nil, err
			}
			if !p.match(")") {
				return nil, nil, nil, p.errorf("expected ) after child relationship subquery")
			}
			if len(query.Fields) == 0 && len(query.Typeofs) == 0 {
				return nil, nil, nil, p.errorf("child relationship subquery requires at least one field")
			}
			childQueries = append(childQueries, ChildQuery{Relationship: childRelationshipNameFromObject(query.Object), Query: query})
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		field, err := p.parseName()
		if err != nil {
			return nil, nil, nil, err
		}
		if strings.EqualFold(field, "TYPEOF") {
			spec, err := p.parseTypeofSpec()
			if err != nil {
				return nil, nil, nil, err
			}
			typeofs = append(typeofs, spec)
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		if strings.EqualFold(field, "FIELDS") && p.match("(") {
			arg, err := p.parseName()
			if err != nil {
				return nil, nil, nil, err
			}
			if !p.match(")") {
				return nil, nil, nil, p.errorf("expected ) after FIELDS(")
			}
			field = "FIELDS(" + strings.ToUpper(arg) + ")"
			fields = append(fields, field)
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		if isSelectFieldFunction(field) && p.match("(") {
			args, err := p.parseFunctionArgs()
			if err != nil {
				return nil, nil, nil, err
			}
			field = strings.ToUpper(field) + "(" + strings.Join(args, ",") + ")"
			if p.matchWord("AS") {
				alias, err := p.parseName()
				if err != nil {
					return nil, nil, nil, err
				}
				field += " " + alias
			} else if tok := p.peek().text; tok != "" && tok != "," && !strings.EqualFold(tok, "FROM") {
				field += " " + p.advance().text
			}
			fields = append(fields, field)
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		isAggregate := isAggregateFunc(field) && p.match("(")
		if isAggregate {
			if strings.EqualFold(field, "COUNT") && p.match(")") {
				field = "COUNT()"
			} else {
				if p.peek().text == ")" {
					return nil, nil, nil, unexpectedToken(strings.ToUpper(field)+"()", "aggregate requires a field")
				}
				arg, err := p.parseName()
				if err != nil {
					return nil, nil, nil, err
				}
				if p.peek().text == "(" {
					message := "nested functional syntax can only be used in the following manner: date_function(convertTimezone(columnName)): " + strings.ToUpper(field)
					return nil, nil, nil, queryError(message, message)
				}
				if !p.match(")") {
					return nil, nil, nil, p.errorf("expected ) after %s(", field)
				}
				field = strings.ToUpper(field) + "(" + arg + ")"
			}
			alias := ""
			if p.matchWord("AS") {
				var err error
				alias, err = p.parseName()
				if err != nil {
					return nil, nil, nil, err
				}
			} else if tok := p.peek().text; tok != "" && tok != "," && !strings.EqualFold(tok, "FROM") {
				alias = p.advance().text
			}
			if alias != "" {
				if field == "COUNT()" {
					return nil, nil, nil, unexpectedToken(alias, "COUNT() does not accept an alias")
				}
				field += " " + alias
			}
			if field == "COUNT()" && (len(fields) > 0 || p.peek().text == ",") {
				return nil, nil, nil, unexpectedToken(",", "COUNT() must be the only selected expression")
			}
		} else if tok := p.peek().text; tok != "" && tok != "," && !strings.EqualFold(tok, "FROM") {
			field += " " + p.advance().text
		}
		fields = append(fields, field)
		if !p.match(",") {
			return fields, childQueries, typeofs, nil
		}
	}
}
func (p *parser) parseNameList() ([]string, error) {
	var names []string
	for {
		name, err := p.parseSelectableName()
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if !p.match(",") {
			return names, nil
		}
	}
}
func (p *parser) parseFunctionArgs() ([]string, error) {
	var args []string
	for {
		var parts []string
		depth := 0
		for {
			tok := p.advance().text
			if tok == "" {
				return nil, p.errorf("expected , or ) in function argument list")
			}
			if tok == ")" && depth == 0 {
				if len(parts) == 0 {
					return nil, p.errorf("expected function argument")
				}
				args = append(args, strings.Join(parts, ""))
				return args, nil
			}
			if tok == "," && depth == 0 {
				if len(parts) == 0 {
					return nil, p.errorf("expected function argument")
				}
				args = append(args, strings.Join(parts, ""))
				break
			}
			switch tok {
			case "(":
				depth++
			case ")":
				depth--
			}
			parts = append(parts, tok)
		}
	}
}
func (p *parser) parseSelectableName() (string, error) {
	name, err := p.parseName()
	if err != nil {
		return "", err
	}
	if !isSelectFieldFunction(name) || !p.match("(") {
		return name, nil
	}
	args, err := p.parseFunctionArgs()
	if err != nil {
		return "", err
	}
	return strings.ToUpper(name) + "(" + strings.Join(args, ",") + ")", nil
}
func (p *parser) parseTypeofSpec() (TypeofSpec, error) {
	relationship, err := p.parseName()
	if err != nil {
		return TypeofSpec{}, err
	}
	spec := TypeofSpec{Relationship: relationship, When: make(map[string][]string)}
	for {
		switch {
		case p.matchWord("WHEN"):
			objectName, err := p.parseName()
			if err != nil {
				return TypeofSpec{}, err
			}
			for previous := range spec.When {
				if strings.EqualFold(previous, objectName) {
					message := "WHEN clause operand [" + objectName + "] is not unique"
					return TypeofSpec{}, queryError(message, message)
				}
			}
			if !p.matchWord("THEN") {
				return TypeofSpec{}, p.errorf("expected THEN in TYPEOF")
			}
			fields, err := p.parseTypeofFieldList()
			if err != nil {
				return TypeofSpec{}, err
			}
			spec.When[objectName] = fields
		case p.matchWord("ELSE"):
			fields, err := p.parseTypeofFieldList()
			if err != nil {
				return TypeofSpec{}, err
			}
			spec.Else = fields
		case p.matchWord("END"):
			return spec, nil
		default:
			return TypeofSpec{}, p.errorf("expected WHEN, ELSE, or END in TYPEOF")
		}
	}
}
func (p *parser) parseTypeofFieldList() ([]string, error) {
	var fields []string
	for {
		if strings.EqualFold(p.peek().text, "FROM") {
			return nil, queryError("Missing 'END' at 'FROM'", "missing END in TYPEOF")
		}
		if p.peek().text == "" || p.peek().text == "," || strings.EqualFold(p.peek().text, "WHEN") || strings.EqualFold(p.peek().text, "ELSE") || strings.EqualFold(p.peek().text, "END") {
			if len(fields) == 0 {
				return nil, p.errorf("TYPEOF branch requires at least one field")
			}
			return fields, nil
		}
		field, err := p.parseName()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		if !p.match(",") {
			continue
		}
	}
}
func (p *parser) parseOrderList() ([]OrderSpec, error) {
	var order []OrderSpec
	for {
		field, err := p.parseSelectableName()
		if err != nil {
			return nil, err
		}
		if field == "" {
			return nil, p.errorf("expected ORDER BY field")
		}
		if isAggregateFunc(field) && p.match("(") {
			args, err := p.parseFunctionArgs()
			if err != nil {
				return nil, err
			}
			field = strings.ToUpper(field) + "(" + strings.Join(args, ",") + ")"
		}
		spec := OrderSpec{Field: field}
		if p.matchWord("ASC") {
			spec.Desc = false
		} else if p.matchWord("DESC") {
			spec.Desc = true
		}
		if p.matchWord("NULLS") {
			switch {
			case p.matchWord("FIRST"):
				spec.Nulls = "FIRST"
			case p.matchWord("LAST"):
				spec.Nulls = "LAST"
			default:
				return nil, queryError("missing value at '"+p.peek().text+"'", "expected FIRST or LAST after NULLS")
			}
		}
		order = append(order, spec)
		if !p.match(",") {
			return order, nil
		}
	}
}
func (p *parser) parseSecurityMode() (string, error) {
	switch {
	case p.matchWord("SECURITY_ENFORCED"):
		return "SECURITY_ENFORCED", nil
	case p.matchWord("USER_MODE"):
		return "USER_MODE", nil
	case p.matchWord("SYSTEM_MODE"):
		return "SYSTEM_MODE", nil
	default:
		return "", p.errorf("expected SECURITY_ENFORCED, USER_MODE, or SYSTEM_MODE after WITH")
	}
}
func isAggregateFunc(name string) bool {
	switch strings.ToUpper(name) {
	case "COUNT", "COUNT_DISTINCT", "SUM", "MIN", "MAX", "AVG", "GROUPING":
		return true
	default:
		return false
	}
}
func isSelectFieldFunction(name string) bool {
	switch strings.ToUpper(name) {
	case "TOLABEL", "FORMAT", "CONVERTCURRENCY",
		"DISTANCE",
		"CALENDAR_MONTH", "CALENDAR_QUARTER", "CALENDAR_YEAR",
		"DAY_IN_MONTH", "DAY_IN_WEEK", "DAY_IN_YEAR", "DAY_ONLY",
		"FISCAL_MONTH", "FISCAL_QUARTER", "FISCAL_YEAR",
		"HOUR_IN_DAY", "WEEK_IN_MONTH", "WEEK_IN_YEAR":
		return true
	default:
		return false
	}
}

type selectFieldExpression struct {
	Func  string
	Args  []string
	Alias string
	Raw   string
}

func (e selectFieldExpression) outputName() string {
	if e.Alias != "" {
		return e.Alias
	}
	return e.Raw
}
func parseSelectFieldExpression(field string) (selectFieldExpression, bool) {
	parts := strings.Fields(field)
	if len(parts) == 0 || len(parts) > 2 {
		return selectFieldExpression{}, false
	}
	raw := parts[0]
	open := strings.Index(raw, "(")
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return selectFieldExpression{}, false
	}
	fn := strings.ToUpper(raw[:open])
	if !isSelectFieldFunction(fn) {
		return selectFieldExpression{}, false
	}
	argsText := raw[open+1 : len(raw)-1]
	if strings.TrimSpace(argsText) == "" {
		return selectFieldExpression{}, false
	}
	args, ok := splitSelectFunctionArgs(argsText)
	if !ok {
		return selectFieldExpression{}, false
	}
	alias := ""
	if len(parts) == 2 {
		alias = parts[1]
	}
	return selectFieldExpression{Func: fn, Args: args, Alias: alias, Raw: raw}, true
}
func validateSelectFieldExpression(org storage.OrgState, definition storage.ObjectDefinition, expr selectFieldExpression, mode string) error {
	if expr.Func == "DISTANCE" {
		return validateDistanceExpression(org, definition, expr, mode)
	}
	if len(expr.Args) != 1 {
		return unsupportedSOQLErrorf("%s currently supports one field argument", expr.Func)
	}
	return validateFieldReference(org, definition, selectFunctionFieldArg(expr.Args[0]), mode)
}

func splitSelectFunctionArgs(argsText string) ([]string, bool) {
	var args []string
	depth := 0
	start := 0
	for i := 0; i < len(argsText); i++ {
		switch argsText[i] {
		case '\'':
			i++
			for i < len(argsText) {
				if argsText[i] == '\'' {
					if i+1 < len(argsText) && argsText[i+1] == '\'' {
						i += 2
						continue
					}
					break
				}
				i++
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, false
			}
		case ',':
			if depth == 0 {
				arg := strings.TrimSpace(argsText[start:i])
				if arg == "" {
					return nil, false
				}
				args = append(args, arg)
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, false
	}
	arg := strings.TrimSpace(argsText[start:])
	if arg == "" {
		return nil, false
	}
	return append(args, arg), true
}

func validateDistanceExpression(org storage.OrgState, definition storage.ObjectDefinition, expr selectFieldExpression, mode string) error {
	if len(expr.Args) != 3 {
		return unsupportedSOQLErrorf("DISTANCE requires two locations and a unit")
	}
	if !distanceUnitSupported(expr.Args[2]) {
		return unsupportedSOQLErrorf("DISTANCE supports 'mi' and 'km' units")
	}
	for _, arg := range expr.Args[:2] {
		if err := validateGeolocationExpressionArg(org, definition, arg, mode); err != nil {
			return err
		}
	}
	return nil
}

func validateGeolocationExpressionArg(org storage.OrgState, definition storage.ObjectDefinition, arg string, mode string) error {
	args, ok := geolocationArgs(arg)
	if ok {
		for _, item := range args {
			if numericLiteral(item) {
				continue
			}
			if err := validateFieldReference(org, definition, item, mode); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validateFieldReference(org, definition, arg, mode); err != nil {
		return err
	}
	return nil
}

func distanceUnitSupported(arg string) bool {
	unit, ok := stringLiteralText(arg)
	if !ok {
		return false
	}
	return strings.EqualFold(unit, "mi") || strings.EqualFold(unit, "km")
}

type geolocationPoint struct {
	lat float64
	lon float64
}

func selectFieldExpressionValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, expr selectFieldExpression) (storage.Value, bool) {
	if expr.Func == "DISTANCE" {
		return distanceExpressionValue(org, definition, record, expr)
	}
	if len(expr.Args) != 1 {
		return storage.Value{}, false
	}
	value, ok := recordValue(org, definition, record, selectFunctionFieldArg(expr.Args[0]))
	if !ok {
		return storage.Value{}, false
	}
	switch expr.Func {
	case "TOLABEL":
		return toLabelValue(org, definition, expr.Args[0], value), true
	case "FORMAT":
		return storage.StringValue(storageValueDisplayString(value)), true
	case "CONVERTCURRENCY":
		return value.Clone(), true
	case "DAY_ONLY":
		if text, ok := storageValueDateText(value); ok {
			return storage.DateValue(text[:10]), true
		}
	case "CALENDAR_MONTH":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.Month())), true
			}
		}
	case "FISCAL_MONTH":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(fiscalMonthNumber(int(parsed.Month()), FiscalYearStartMonth(org)))), true
			}
		}
	case "CALENDAR_QUARTER":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64((int(parsed.Month())-1)/3 + 1)), true
			}
		}
	case "FISCAL_QUARTER":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(fiscalQuarterNumber(int(parsed.Month()), FiscalYearStartMonth(org)))), true
			}
		}
	case "CALENDAR_YEAR":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.Year())), true
			}
		}
	case "FISCAL_YEAR":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				startMonth, useStartYear := fiscalYearSettings(org)
				return storage.IntegerValue(int64(fiscalYearNumber(parsed, startMonth, useStartYear))), true
			}
		}
	case "DAY_IN_MONTH":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.Day())), true
			}
		}
	case "DAY_IN_WEEK":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				weekday := int(parsed.Weekday()) + 1
				return storage.IntegerValue(int64(weekday)), true
			}
		}
	case "DAY_IN_YEAR":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.YearDay())), true
			}
		}
	case "HOUR_IN_DAY":
		if value.Kind == storage.ValueDateTime && len(value.String) >= 13 {
			if parsed, err := time.Parse(time.RFC3339, normalizeDateTime(value.String)); err == nil {
				return storage.IntegerValue(int64(parsed.Hour())), true
			}
		}
	case "WEEK_IN_MONTH":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64((parsed.Day()-1)/7 + 1)), true
			}
		}
	case "WEEK_IN_YEAR":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				_, week := parsed.ISOWeek()
				return storage.IntegerValue(int64(week)), true
			}
		}
	}
	return storage.NullValue(), true
}

func distanceExpressionValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, expr selectFieldExpression) (storage.Value, bool) {
	if len(expr.Args) != 3 {
		return storage.Value{}, false
	}
	left, ok := geolocationPointValue(org, definition, record, expr.Args[0])
	if !ok {
		return storage.NullValue(), true
	}
	right, ok := geolocationPointValue(org, definition, record, expr.Args[1])
	if !ok {
		return storage.NullValue(), true
	}
	unit, ok := stringLiteralText(expr.Args[2])
	if !ok {
		return storage.Value{}, false
	}
	distance := haversineDistance(left, right)
	switch {
	case strings.EqualFold(unit, "km"):
		return storage.DecimalValue(floatDecimalString(distance)), true
	case strings.EqualFold(unit, "mi"):
		return storage.DecimalValue(floatDecimalString(distance * 0.621371192237334)), true
	default:
		return storage.Value{}, false
	}
}

func geolocationPointValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, arg string) (geolocationPoint, bool) {
	args, ok := geolocationArgs(arg)
	if ok {
		lat, ok := geolocationNumberValue(org, definition, record, args[0])
		if !ok {
			return geolocationPoint{}, false
		}
		lon, ok := geolocationNumberValue(org, definition, record, args[1])
		if !ok {
			return geolocationPoint{}, false
		}
		return geolocationPoint{lat: lat, lon: lon}, true
	}
	lat, lon, ok := locationComponentFieldNames(definition, org.Namespace, arg)
	if !ok {
		return geolocationPoint{}, false
	}
	latValue, ok := geolocationNumberValue(org, definition, record, lat)
	if !ok {
		return geolocationPoint{}, false
	}
	lonValue, ok := geolocationNumberValue(org, definition, record, lon)
	if !ok {
		return geolocationPoint{}, false
	}
	return geolocationPoint{lat: latValue, lon: lonValue}, true
}

func geolocationArgs(arg string) ([]string, bool) {
	arg = strings.TrimSpace(arg)
	open := strings.Index(arg, "(")
	if open <= 0 || !strings.HasSuffix(arg, ")") || !strings.EqualFold(strings.TrimSpace(arg[:open]), "GEOLOCATION") {
		return nil, false
	}
	args, ok := splitSelectFunctionArgs(arg[open+1 : len(arg)-1])
	if !ok || len(args) != 2 {
		return nil, false
	}
	return args, true
}

func geolocationNumberValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, arg string) (float64, bool) {
	if number, ok := parseFloatLiteral(arg); ok {
		return number, true
	}
	value, ok := recordValue(org, definition, record, arg)
	if !ok || value.Kind == storage.ValueNull {
		return 0, false
	}
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		number, err := strconv.ParseFloat(value.Decimal, 64)
		return number, err == nil
	case storage.ValueString:
		number, err := strconv.ParseFloat(strings.TrimSpace(value.String), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func locationComponentFieldNames(definition storage.ObjectDefinition, namespace string, field string) (string, string, bool) {
	canonical, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return "", "", false
	}
	fieldDef, ok := definition.Fields[canonical]
	if !ok || fieldDef.Type != storage.FieldLocation {
		return "", "", false
	}
	base := strings.TrimSuffix(canonical, "__c")
	latCandidates := []string{base + "__Latitude__s", base + "__latitude__s"}
	lonCandidates := []string{base + "__Longitude__s", base + "__longitude__s"}
	lat, ok := firstResolvedFieldName(definition, namespace, latCandidates)
	if !ok {
		return "", "", false
	}
	lon, ok := firstResolvedFieldName(definition, namespace, lonCandidates)
	if !ok {
		return "", "", false
	}
	return lat, lon, true
}

func firstResolvedFieldName(definition storage.ObjectDefinition, namespace string, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if field, ok := storage.ResolveFieldName(definition, namespace, candidate); ok {
			return field, true
		}
	}
	return "", false
}

func haversineDistance(left, right geolocationPoint) float64 {
	const earthRadiusKM = 6371.0088
	lat1 := left.lat * math.Pi / 180
	lat2 := right.lat * math.Pi / 180
	dlat := (right.lat - left.lat) * math.Pi / 180
	dlon := (right.lon - left.lon) * math.Pi / 180
	sinDlat := math.Sin(dlat / 2)
	sinDlon := math.Sin(dlon / 2)
	a := sinDlat*sinDlat + math.Cos(lat1)*math.Cos(lat2)*sinDlon*sinDlon
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func numericLiteral(arg string) bool {
	_, ok := parseFloatLiteral(arg)
	return ok
}

func parseFloatLiteral(arg string) (float64, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(arg, 64)
	return number, err == nil
}

func stringLiteralText(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if !strings.HasPrefix(arg, "'") || !strings.HasSuffix(arg, "'") {
		return "", false
	}
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(arg, "'"), "'"), "''", "'"), true
}

func floatDecimalString(value float64) string {
	text := strconv.FormatFloat(value, 'f', 6, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func selectFunctionFieldArg(arg string) string {
	arg = strings.TrimSpace(arg)
	open := strings.Index(arg, "(")
	if open <= 0 || !strings.HasSuffix(arg, ")") {
		return arg
	}
	fn := strings.ToUpper(strings.TrimSpace(arg[:open]))
	if fn != "CONVERTTIMEZONE" {
		return arg
	}
	inner := strings.TrimSpace(arg[open+1 : len(arg)-1])
	if inner == "" || strings.ContainsAny(inner, ",()") {
		return arg
	}
	return inner
}
func toLabelValue(org storage.OrgState, definition storage.ObjectDefinition, field string, value storage.Value) storage.Value {
	fields, ok := fieldDefinitionsForReference(org, definition, field)
	if !ok || len(fields) == 0 {
		return value.Clone()
	}
	text := storageValueDisplayString(value)
	for _, fieldDef := range fields {
		for _, option := range fieldDef.PicklistValues {
			if option.Value == text {
				if option.Label != "" {
					return storage.StringValue(option.Label)
				}
				return storage.StringValue(option.Value)
			}
		}
	}
	return value.Clone()
}
func storageValueDisplayString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueNull:
		return ""
	case storage.ValueDate:
		if parsed, err := time.Parse("2006-01-02", value.String); err == nil {
			return fmt.Sprintf("%d/%d/%d", int(parsed.Month()), parsed.Day(), parsed.Year())
		}
		return value.String
	case storage.ValueString, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		return strconv.FormatBool(value.Boolean)
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	default:
		return ""
	}
}
func storageValueDateText(value storage.Value) (string, bool) {
	if (value.Kind == storage.ValueDate || value.Kind == storage.ValueDateTime) && len(value.String) >= 10 {
		return value.String, true
	}
	return "", false
}
func normalizeDateTime(text string) string {
	if strings.HasSuffix(text, "Z") || hasDateTimeZoneOffset(text) {
		return text
	}
	return text + "Z"
}
func hasDateTimeZoneOffset(text string) bool {
	if len(text) >= len("2006-01-02T15:04:05-0700") {
		offset := text[len(text)-5:]
		if (offset[0] == '+' || offset[0] == '-') && isASCIIDigit(offset[1]) && isASCIIDigit(offset[2]) && isASCIIDigit(offset[3]) && isASCIIDigit(offset[4]) {
			return true
		}
	}
	if len(text) < len("2006-01-02T15:04:05-07:00") {
		return false
	}
	offset := text[len(text)-6:]
	return (offset[0] == '+' || offset[0] == '-') &&
		offset[3] == ':' &&
		isASCIIDigit(offset[1]) &&
		isASCIIDigit(offset[2]) &&
		isASCIIDigit(offset[4]) &&
		isASCIIDigit(offset[5])
}
func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
func aggregateSpecs(fields []string) ([]Aggregate, error) {
	var aggregates []Aggregate
	for _, field := range fields {
		aggregate, ok, err := parseAggregateField(field)
		if err != nil {
			return nil, err
		}
		if ok {
			aggregates = append(aggregates, aggregate)
		}
	}
	return aggregates, nil
}
func validateAggregateOffset(query Query) error {
	if len(query.GroupBy) == 0 && len(query.Aggregates) > 0 && !query.Count {
		message := "Non-grouped query that uses overall aggregate functions cannot also use OFFSET"
		return unexpectedQueryError(message, message)
	}
	return nil
}

func validateAggregateQuery(query Query) error {
	if err := validateHavingSubqueries(query.Having); err != nil {
		return err
	}
	if strings.EqualFold(query.GroupMode, "ROLLUP") && len(query.GroupBy) > 3 {
		return fmt.Errorf("soql: GROUP BY ROLLUP must contain 3 fields or less")
	}
	if strings.EqualFold(query.GroupMode, "CUBE") && len(query.GroupBy) > 3 {
		return fmt.Errorf("soql: GROUP BY CUBE must contain 3 fields or less")
	}
	seenGroups := make(map[string]bool, len(query.GroupBy))
	for _, field := range query.GroupBy {
		key := strings.ToLower(field)
		if seenGroups[key] {
			message := "duplicate grouping expression: " + field
			return unexpectedQueryError(message, message)
		}
		seenGroups[key] = true
	}
	if len(query.Aggregates) == 0 {
		// Also validate callers that construct Query values instead of parsing text.
		aggregates, err := aggregateSpecs(query.Fields)
		if err != nil {
			return err
		}
		query.Aggregates = aggregates
	}
	if len(query.GroupBy) > 0 && strings.TrimSpace(query.GroupMode) == "" {
		for _, aggregate := range query.Aggregates {
			if aggregate.Field == "" || strings.EqualFold(aggregate.Func, "GROUPING") {
				continue
			}
			for _, groupedField := range query.GroupBy {
				if strings.EqualFold(aggregate.Field, groupedField) {
					message := "Grouped field should not be aggregated: " + aggregate.Field
					return &QueryError{Message: message, Err: fmt.Errorf("%s", message)}
				}
			}
		}
	}
	if query.HasOffset || query.OffsetBind != "" {
		if err := validateAggregateOffset(query); err != nil {
			return err
		}
	}
	if (query.HasLimit || query.LimitBind != "") && len(query.GroupBy) == 0 && len(query.Aggregates) > 0 {
		bareCount := false
		if len(query.Fields) == 1 && len(query.Aggregates) == 1 {
			selected, aggregate, err := parseAggregateField(query.Fields[0])
			if err != nil {
				return err
			}
			bareCount = aggregate && strings.EqualFold(selected.Func, "COUNT") && selected.Field == "" && selected.Alias == "" &&
				strings.EqualFold(query.Aggregates[0].Func, "COUNT") && query.Aggregates[0].Field == "" && query.Aggregates[0].Alias == ""
		}
		if !bareCount {
			return &QueryError{
				NonGroupedAggregateLimit: true,
				Err:                      fmt.Errorf("soql: non-grouped query with aggregate functions cannot also use LIMIT"),
			}
		}
	}
	if len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 {
		if query.Having != nil {
			return fmt.Errorf("soql: GROUP BY and HAVING require aggregate fields")
		}
		return nil
	}
	return validateGroupedSelectedFields(query, true)
}

func validateGroupedSelectedFields(query Query, deferFieldsExpansion bool) error {
	for _, field := range query.Fields {
		if _, unexpanded := fieldsFunctionMode(field); unexpanded && deferFieldsExpansion {
			continue
		}
		aggregate, ok, err := parseAggregateField(field)
		if err != nil {
			return err
		}
		if ok {
			if aggregate.Func == "GROUPING" {
				if len(query.GroupBy) == 0 {
					return fmt.Errorf("soql: GROUPING requires GROUP BY")
				}
				if !containsName(query.GroupBy, aggregate.Field) {
					return fmt.Errorf("soql: GROUPING field %s must be grouped", aggregate.Field)
				}
			}
			continue
		}
		if !containsName(query.GroupBy, groupingComparableField(field)) && !dateFunctionHasRawGroupCandidate(field, query.GroupBy) {
			return unexpectedQueryError("Field must be grouped or aggregated: "+field, fmt.Sprintf("field %s must be grouped or aggregated", field))
		}
	}
	return nil
}

func validateDateFunctionGrouping(query Query, org storage.OrgState, definition *storage.ObjectDefinition) error {
	for _, selected := range query.Fields {
		expr, ok := parseSelectFieldExpression(selected)
		if !ok || !isSOQLDateFieldFunction(expr.Func) {
			continue
		}
		if len(expr.Args) != 1 {
			return unsupportedSOQLErrorf("%s currently supports one field argument", expr.Func)
		}

		matchedExpression := false
		for _, grouped := range query.GroupBy {
			groupExpr, ok := parseSelectFieldExpression(grouped)
			if !ok || !isSOQLDateFieldFunction(groupExpr.Func) || len(groupExpr.Args) != 1 {
				continue
			}
			if strings.EqualFold(expr.Func, groupExpr.Func) && strings.EqualFold(strings.TrimSpace(expr.Args[0]), strings.TrimSpace(groupExpr.Args[0])) {
				matchedExpression = true
				break
			}
		}
		if matchedExpression {
			continue
		}

		fieldArg, simpleFieldArg := dateFunctionRawFieldArgument(expr)
		matchedRawDate := false
		if simpleFieldArg {
			for _, grouped := range query.GroupBy {
				if !sameSOQLFieldReference(org, definition, grouped, fieldArg) {
					continue
				}
				if definition == nil {
					// Parsing lacks schema types. Accept this syntactic candidate and
					// resolve the Date-only exception in execution-time validation.
					matchedRawDate = true
					break
				}
				fields, ok := dateGroupingFieldDefinitions(org, *definition, fieldArg)
				if ok && len(fields) > 0 {
					allDate := true
					allDateTime := true
					for _, field := range fields {
						allDate = allDate && field.Type == storage.FieldDate
						allDateTime = allDateTime && field.Type == storage.FieldDateTime
					}
					if allDate {
						matchedRawDate = true
						break
					}
					if allDateTime {
						return fmt.Errorf("soql: DateTime field %s requires matching date function %s in GROUP BY", fieldArg, expr.Func)
					}
				}
			}
		}
		if !matchedRawDate {
			return fmt.Errorf("soql: date function %s requires a matching GROUP BY expression", expr.Raw)
		}
	}
	return nil
}

func isSOQLDateFieldFunction(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "CALENDAR_MONTH", "CALENDAR_QUARTER", "CALENDAR_YEAR",
		"DAY_IN_MONTH", "DAY_IN_WEEK", "DAY_IN_YEAR", "DAY_ONLY",
		"FISCAL_MONTH", "FISCAL_QUARTER", "FISCAL_YEAR",
		"HOUR_IN_DAY", "WEEK_IN_MONTH", "WEEK_IN_YEAR":
		return true
	default:
		return false
	}
}

func dateFunctionHasRawGroupCandidate(field string, groupBy []string) bool {
	expr, ok := parseSelectFieldExpression(field)
	if !ok || !isSOQLDateFieldFunction(expr.Func) || len(expr.Args) != 1 {
		return false
	}
	fieldArg, simpleFieldArg := dateFunctionRawFieldArgument(expr)
	return simpleFieldArg && containsName(groupBy, fieldArg)
}

func dateFunctionRawFieldArgument(expr selectFieldExpression) (string, bool) {
	if len(expr.Args) != 1 {
		return "", false
	}
	field := strings.TrimSpace(expr.Args[0])
	if field == "" || strings.ContainsAny(field, "()") {
		return "", false
	}
	return field, true
}

func sameSOQLFieldReference(org storage.OrgState, definition *storage.ObjectDefinition, left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if definition != nil {
		if base, ok := stripQualifiedCurrentObjectField(org, *definition, left); ok {
			left = base
		}
		if base, ok := stripQualifiedCurrentObjectField(org, *definition, right); ok {
			right = base
		}
		resolvedLeft, leftOK := storage.ResolveFieldName(*definition, org.Namespace, left)
		resolvedRight, rightOK := storage.ResolveFieldName(*definition, org.Namespace, right)
		if leftOK && rightOK {
			return strings.EqualFold(resolvedLeft, resolvedRight)
		}
	}
	return strings.EqualFold(left, right)
}

func dateGroupingFieldDefinitions(org storage.OrgState, definition storage.ObjectDefinition, fieldName string) ([]storage.Field, bool) {
	if base, ok := stripQualifiedCurrentObjectField(org, definition, fieldName); ok {
		fieldName = base
	}
	return fieldDefinitionsForReference(org, definition, fieldName)
}

func validateHavingSubqueries(condition *Condition) error {
	if condition == nil {
		return nil
	}
	if condition.Subquery != nil {
		return fmt.Errorf("soql: HAVING does not support semi-join or anti-join subqueries")
	}
	for i := range condition.And {
		if err := validateHavingSubqueries(&condition.And[i]); err != nil {
			return err
		}
	}
	for i := range condition.Or {
		if err := validateHavingSubqueries(&condition.Or[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateAggregateAliases(query Query) error {
	seen := map[string]bool{}
	for _, aggregate := range query.Aggregates {
		if aggregate.Alias == "" {
			continue
		}
		aliasKey := strings.ToLower(aggregate.Alias)
		if seen[aliasKey] {
			return fmt.Errorf("soql: duplicate aggregate alias %s", aggregate.Alias)
		}
		seen[aliasKey] = true
		for exprIndex := range query.Aggregates {
			if strings.EqualFold(aggregate.Alias, fmt.Sprintf("expr%d", exprIndex)) {
				return fmt.Errorf("soql: aggregate alias %s conflicts with generated aggregate field", aggregate.Alias)
			}
		}
		for _, groupField := range query.GroupBy {
			if strings.EqualFold(aggregate.Alias, groupField) {
				return fmt.Errorf("soql: aggregate alias %s conflicts with grouped field %s", aggregate.Alias, groupField)
			}
		}
	}
	return nil
}
func groupingComparableField(field string) string {
	if expr, ok := parseSelectFieldExpression(field); ok {
		return expr.Raw
	}
	if raw, _, ok := splitSelectFieldAlias(field); ok {
		return raw
	}
	return field
}
func hasPrefixFold(value, prefix string) bool {
	if len(prefix) > len(value) {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}
func hasSuffixFold(value, suffix string) bool {
	if len(suffix) > len(value) {
		return false
	}
	return strings.EqualFold(value[len(value)-len(suffix):], suffix)
}
func splitSelectFieldAlias(field string) (string, string, bool) {
	parts := strings.Fields(field)
	if len(parts) != 2 {
		return "", "", false
	}
	if strings.Contains(parts[0], "(") {
		return "", "", false
	}
	return parts[0], parts[1], true
}
func aggregateExprMap(aggregates []Aggregate) map[string]string {
	out := make(map[string]string, len(aggregates))
	for i, aggregate := range aggregates {
		out[aggregateExpression(aggregate)] = fmt.Sprintf("expr%d", i)
	}
	return out
}
func aggregateExpression(aggregate Aggregate) string {
	if aggregate.Field == "" {
		return aggregate.Func + "()"
	}
	return aggregate.Func + "(" + aggregate.Field + ")"
}
func rewriteHavingAggregates(condition Condition, query *Query) Condition {
	condition = rewriteConditionAggregates(condition, aggregateExprMap(query.Aggregates))
	hidden := make(map[string]string, len(query.HavingAggregates))
	for _, aggregate := range query.HavingAggregates {
		if aggregate.Alias != "" {
			hidden[aggregateExpression(aggregate)] = aggregate.Alias
		}
	}
	return rewriteUnselectedHavingAggregates(condition, &query.HavingAggregates, hidden)
}
func rewriteUnselectedHavingAggregates(condition Condition, aggregates *[]Aggregate, aliases map[string]string) Condition {
	if condition.Field != "" {
		if aggregate, ok, err := parseAggregateField(condition.Field); err == nil && ok {
			expression := aggregateExpression(aggregate)
			alias, ok := aliases[expression]
			if !ok {
				alias = fmt.Sprintf("\x00havingAggregate%d", len(*aggregates))
				aggregate.Alias = alias
				*aggregates = append(*aggregates, aggregate)
				aliases[expression] = alias
			}
			condition.Field = alias
			condition.RewrittenAggregate = true
		}
	}
	for i := range condition.And {
		condition.And[i] = rewriteUnselectedHavingAggregates(condition.And[i], aggregates, aliases)
	}
	for i := range condition.Or {
		condition.Or[i] = rewriteUnselectedHavingAggregates(condition.Or[i], aggregates, aliases)
	}
	return condition
}
func rewriteConditionAggregates(condition Condition, aliases map[string]string) Condition {
	if condition.Field != "" {
		if alias, ok := aliases[condition.Field]; ok {
			condition.Field = alias
			condition.RewrittenAggregate = true
		}
	}
	for i := range condition.And {
		condition.And[i] = rewriteConditionAggregates(condition.And[i], aliases)
	}
	for i := range condition.Or {
		condition.Or[i] = rewriteConditionAggregates(condition.Or[i], aliases)
	}
	return condition
}
func containsName(values []string, want string) bool {
	wantNormalized := comparableSOQLName(want)
	for _, value := range values {
		if strings.EqualFold(value, want) || strings.EqualFold(comparableSOQLName(value), wantNormalized) {
			return true
		}
	}
	return false
}
func comparableSOQLName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx+1 < len(name) {
		name = name[idx+1:]
	}
	return storage.StripAnyNamespaceToken(name)
}
func findAggregateFieldByComparableName(fields map[string]storage.Value, raw string) (storage.Value, bool) {
	want := comparableSOQLName(raw)
	for key, value := range fields {
		if strings.EqualFold(comparableSOQLName(key), want) {
			return value, true
		}
	}
	return storage.Value{}, false
}
func parseAggregateField(field string) (Aggregate, bool, error) {
	alias := ""
	parts := strings.Fields(field)
	if len(parts) > 2 {
		return Aggregate{}, false, fmt.Errorf("soql: invalid aggregate field %s", field)
	}
	if len(parts) == 2 {
		field = parts[0]
		alias = parts[1]
	}
	open := strings.Index(field, "(")
	if open < 0 || !strings.HasSuffix(field, ")") {
		return Aggregate{}, false, nil
	}
	fn := strings.ToUpper(field[:open])
	if !isAggregateFunc(fn) {
		return Aggregate{}, false, nil
	}
	arg := strings.TrimSpace(field[open+1 : len(field)-1])
	if fn == "GROUPING" {
		if arg == "" {
			return Aggregate{}, false, fmt.Errorf("soql: GROUPING requires a field")
		}
		return Aggregate{Func: fn, Field: arg, Alias: alias}, true, nil
	}
	if fn == "COUNT" && arg == "" {
		return Aggregate{Func: fn, Alias: alias}, true, nil
	}
	if arg == "" {
		return Aggregate{}, false, fmt.Errorf("soql: %s requires a field", fn)
	}
	return Aggregate{Func: fn, Field: arg, Alias: alias}, true, nil
}
func (p *parser) parseOrCondition() (Condition, error) {
	left, err := p.parseAndCondition()
	if err != nil {
		return Condition{}, err
	}
	var ors []Condition
	for p.matchWord("OR") {
		right, err := p.parseAndCondition()
		if err != nil {
			return Condition{}, err
		}
		ors = append(ors, right)
	}
	if len(ors) == 0 {
		return left, nil
	}
	return Condition{Or: append([]Condition{left}, ors...)}, nil
}
func (p *parser) parseAndCondition() (Condition, error) {
	left, err := p.parsePrimaryCondition()
	if err != nil {
		return Condition{}, err
	}
	var ands []Condition
	for p.matchWord("AND") {
		right, err := p.parsePrimaryCondition()
		if err != nil {
			return Condition{}, err
		}
		ands = append(ands, right)
	}
	if len(ands) == 0 {
		return left, nil
	}
	return Condition{And: append([]Condition{left}, ands...), SourceAnd: true}, nil
}
func (p *parser) parsePrimaryCondition() (Condition, error) {
	if p.matchWord("NOT") {
		cond, err := p.parsePrimaryCondition()
		if err != nil {
			return Condition{}, err
		}
		cond.Not = true
		// Parentheses around the operand do not enclose the NOT expression.
		cond.Parenthesized = false
		return cond, nil
	}
	if p.match("(") {
		cond, err := p.parseOrCondition()
		if err != nil {
			return Condition{}, err
		}
		if !p.match(")") {
			return Condition{}, p.errorf("expected ) after condition")
		}
		cond.Parenthesized = true
		return cond, nil
	}
	field, err := p.parseConditionField()
	if err != nil {
		return Condition{}, err
	}
	op, err := p.parseOperator()
	if err != nil {
		return Condition{}, err
	}
	if p.bindColumns != nil {
		operandStart := p.pos
		defer func() {
			for _, tok := range p.tokens[operandStart:p.pos] {
				if len(tok.text) > 1 && tok.text[0] == ':' {
					if _, exists := p.bindColumns[tok.bindOffset]; !exists {
						p.bindColumns[tok.bindOffset] = BindColumn{Object: p.bindObject, Field: field}
					}
				}
			}
		}()
	}
	if op == "IN" || op == "NOT IN" {
		values, value2s, ranges, subquery, err := p.parseInOperand()
		if err != nil {
			return Condition{}, err
		}
		if anyRange(ranges) {
			conditions := make([]Condition, 0, len(values))
			for i, value := range values {
				if op == "NOT IN" && value.Kind == storage.ValueNull {
					continue
				}
				itemOp := "="
				if op == "NOT IN" {
					itemOp = "!="
				}
				item := Condition{Field: field, Op: itemOp, Value: value, Range: ranges[i], DateLiteralTimeZoneID: p.dateLiteralTimeZoneID}
				if ranges[i] {
					item.Value2 = value2s[i]
				}
				conditions = append(conditions, item)
			}
			if op == "NOT IN" {
				return Condition{And: conditions}, nil
			}
			return Condition{Or: conditions}, nil
		}
		return Condition{Field: field, Op: op, Values: values, Subquery: subquery}, nil
	}
	parenthesizedValue := p.match("(")
	valueToken := p.advance().text
	if valueToken == "" {
		return Condition{}, unexpectedToken("", "expected WHERE value")
	}
	valueToken = p.literalToken(valueToken)
	allowLikeEscapes := op == "LIKE" || op == "NOT LIKE"
	value, value2, isRange, err := literalAtWithMode(valueToken, p.now, p.fiscalYearStartMonth, allowLikeEscapes)
	if err != nil {
		return Condition{}, err
	}
	if parenthesizedValue {
		values := []storage.Value{value}
		ranges := []bool{isRange}
		for p.match(",") {
			tok := p.advance().text
			if tok == "" {
				return Condition{}, p.errorf("expected WHERE value")
			}
			tok = p.literalToken(tok)
			nextValue, _, nextRange, err := literalAtWithMode(tok, p.now, p.fiscalYearStartMonth, allowLikeEscapes)
			if err != nil {
				return Condition{}, err
			}
			values = append(values, nextValue)
			ranges = append(ranges, nextRange)
		}
		if !p.match(")") {
			return Condition{}, p.errorf("expected ) after WHERE value")
		}
		if op == "INCLUDES" || op == "EXCLUDES" {
			return Condition{Field: field, Op: op, Value: value, Values: values}, nil
		}
		if len(values) > 1 && (op == "LIKE" || op == "NOT LIKE") {
			conditions := make([]Condition, 0, len(values))
			for i, item := range values {
				conditions = append(conditions, Condition{Field: field, Op: op, Value: item, Range: ranges[i], DateLiteralTimeZoneID: p.dateLiteralTimeZoneID})
			}
			if op == "NOT LIKE" {
				return Condition{And: conditions}, nil
			}
			return Condition{Or: conditions}, nil
		}
	}
	return Condition{Field: field, Op: op, Value: value, Value2: value2, Range: isRange, DateLiteralTimeZoneID: p.dateLiteralTimeZoneID}, nil
}

func anyRange(ranges []bool) bool {
	for _, item := range ranges {
		if item {
			return true
		}
	}
	return false
}

func (p *parser) parseConditionField() (string, error) {
	field, err := p.parseName()
	if err != nil {
		return "", err
	}
	if isSelectFieldFunction(field) && p.match("(") {
		args, err := p.parseFunctionArgs()
		if err != nil {
			return "", err
		}
		return strings.ToUpper(field) + "(" + strings.Join(args, ",") + ")", nil
	}
	if isAggregateFunc(field) && p.match("(") {
		if strings.EqualFold(field, "COUNT") && p.match(")") {
			return "COUNT()", nil
		}
		arg, err := p.parseName()
		if err != nil {
			return "", err
		}
		if !p.match(")") {
			return "", p.errorf("expected ) after %s(", field)
		}
		return strings.ToUpper(field) + "(" + arg + ")", nil
	}
	return field, nil
}
func (p *parser) literalToken(tok string) string {
	// The lexer leaves a numeric suffix separate so numbered date literals
	// retain their colon. In a value position, a leading colon is an Apex bind.
	if tok == ":" {
		next := p.peek().text
		if next != "" && strings.Trim(next, "0123456789") == "" {
			return tok + p.advance().text
		}
	}
	tok = p.signedLiteralToken(tok)
	if !hasNumberedDateLiteralPrefix(tok) || p.peek().text != ":" {
		return tok
	}
	colon := p.advance().text
	next := p.peek().text
	if next == "" || soqlLiteralStopBytes[next[0]] {
		return tok + colon
	}
	p.advance()
	return tok + colon + next
}
func (p *parser) signedLiteralToken(tok string) string {
	if tok != "-" && tok != "+" {
		return tok
	}
	next := p.peek().text
	if next == "" || soqlLiteralStopBytes[next[0]] {
		return tok
	}
	p.advance()
	return tok + next
}
func hasNumberedDateLiteralPrefix(text string) bool {
	switch strings.ToUpper(text) {
	case "LAST_N_DAYS", "NEXT_N_DAYS", "N_DAYS_AGO",
		"LAST_N_WEEKS", "NEXT_N_WEEKS", "N_WEEKS_AGO",
		"LAST_N_MONTHS", "NEXT_N_MONTHS", "N_MONTHS_AGO",
		"LAST_N_QUARTERS", "NEXT_N_QUARTERS", "N_QUARTERS_AGO",
		"LAST_N_YEARS", "NEXT_N_YEARS", "N_YEARS_AGO",
		"LAST_N_FISCAL_QUARTERS", "NEXT_N_FISCAL_QUARTERS", "N_FISCAL_QUARTERS_AGO",
		"LAST_N_FISCAL_YEARS", "NEXT_N_FISCAL_YEARS", "N_FISCAL_YEARS_AGO":
		return true
	default:
		return false
	}
}
func (p *parser) parseOperator() (string, error) {
	tok := p.advance().text
	if tok == "<" && p.match(">") {
		return "!=", nil
	}
	switch tok {
	case "=", "!=", ">", "<", ">=", "<=":
		return tok, nil
	}
	// Word operators
	word := tok
	if strings.EqualFold(word, "LIKE") {
		return "LIKE", nil
	}
	if strings.EqualFold(word, "INCLUDES") {
		return "INCLUDES", nil
	}
	if strings.EqualFold(word, "EXCLUDES") {
		return "EXCLUDES", nil
	}
	if strings.EqualFold(word, "IN") {
		return "IN", nil
	}
	if strings.EqualFold(word, "NOT") {
		if p.matchWord("IN") {
			return "NOT IN", nil
		}
		if p.matchWord("LIKE") {
			return "NOT LIKE", nil
		}
		return "", p.errorf("expected IN or LIKE after NOT")
	}
	return "", p.errorf("unsupported WHERE operator %q", tok)
}
func (p *parser) parseInOperand() ([]storage.Value, []storage.Value, []bool, *Query, error) {
	if !p.match("(") {
		tok := p.advance().text
		if tok == "" {
			return nil, nil, nil, nil, p.errorf("expected value after IN")
		}
		tok = p.literalToken(tok)
		value, value2, isRange, err := literalAtWithFiscalYearStartMonth(tok, p.now, p.fiscalYearStartMonth)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return []storage.Value{value}, []storage.Value{value2}, []bool{isRange}, nil, nil
	}
	if strings.EqualFold(p.peek().text, "SELECT") {
		query, err := p.parseQuery()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(query.Fields) != 1 {
			return nil, nil, nil, nil, p.errorf("semi-join subquery must select one field")
		}
		if !p.match(")") {
			return nil, nil, nil, nil, p.errorf("expected ) after semi-join subquery")
		}
		return nil, nil, nil, &query, nil
	}
	var values []storage.Value
	var value2s []storage.Value
	var ranges []bool
	if p.match(")") {
		return values, value2s, ranges, nil, nil
	}
	for {
		tok := p.advance().text
		if tok == "" {
			return nil, nil, nil, nil, p.errorf("expected value in IN list")
		}
		tok = p.literalToken(tok)
		value, value2, isRange, err := literalAtWithFiscalYearStartMonth(tok, p.now, p.fiscalYearStartMonth)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		values = append(values, value)
		value2s = append(value2s, value2)
		ranges = append(ranges, isRange)
		if p.match(")") {
			break
		}
		if !p.match(",") {
			return nil, nil, nil, nil, p.errorf("expected , or ) in IN list")
		}
	}
	return values, value2s, ranges, nil, nil
}
func (p *parser) parseName() (string, error) {
	name := p.advance().text
	if name == "" {
		return "", unexpectedToken("", "expected name")
	}
	for p.match(".") {
		part := p.advance().text
		if part == "" {
			return "", p.errorf("expected name after .")
		}
		name += "." + part
	}
	return name, nil
}
func (p *parser) parseInt() (int, error) {
	text := p.advance().text
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return 0, p.errorf("expected non-negative integer")
	}
	return value, nil
}

func (p *parser) parseIntOrBind(clause string) (int, string, error) {
	text := p.advance().text
	if len(text) > 1 && text[0] == ':' {
		return 0, text[1:], nil
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		if err == nil && value < 0 {
			message := "Limit must be a non-negative value"
			if clause == "OFFSET" {
				message = "SOQL offset must be a non-negative value"
			}
			return 0, "", queryError(message, "expected non-negative integer or bind")
		}
		return 0, "", unexpectedToken(text, "expected non-negative integer or bind")
	}
	return value, "", nil
}

func isSOQLBindName(name string) bool {
	for index, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return name != ""
}
func (p *parser) match(text string) bool {
	if p.peek().text != text {
		return false
	}
	p.pos++
	return true
}
func (p *parser) matchWord(text string) bool {
	if !strings.EqualFold(p.peek().text, text) {
		return false
	}
	p.pos++
	return true
}
func (p *parser) peek() token {
	return p.tokens[p.pos]
}
func (p *parser) advance() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}
func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("soql: "+format, args...)
}
func literalAt(text string, now time.Time) (storage.Value, storage.Value, bool, error) {
	return literalAtWithFiscalYearStartMonth(text, now, 1)
}

func literalAtWithFiscalYearStartMonth(text string, now time.Time, fiscalYearStartMonth int) (storage.Value, storage.Value, bool, error) {
	return literalAtWithMode(text, now, fiscalYearStartMonth, false)
}

func literalAtWithMode(text string, now time.Time, fiscalYearStartMonth int, allowLikeEscapes bool) (storage.Value, storage.Value, bool, error) {
	if start, end, ok := dateLiteralWithFiscalYearStartMonth(text, now, fiscalYearStartMonth); ok {
		return start, end, true, nil
	}
	if isKnownUnsupportedDateLiteral(text) {
		return storage.Value{}, storage.Value{}, false, unsupportedSOQLErrorf("date literal %s is not supported", strings.ToUpper(text))
	}
	switch {
	case strings.EqualFold(text, "null"):
		return storage.NullValue(), storage.Value{}, false, nil
	case strings.EqualFold(text, "true"):
		return storage.BooleanValue(true), storage.Value{}, false, nil
	case strings.EqualFold(text, "false"):
		return storage.BooleanValue(false), storage.Value{}, false, nil
	case strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'"):
		inner := strings.TrimSuffix(strings.TrimPrefix(text, "'"), "'")
		value, err := unescapeSOQLStringLiteral(inner, allowLikeEscapes)
		if err != nil {
			return storage.Value{}, storage.Value{}, false, err
		}
		return storage.StringValue(value), storage.Value{}, false, nil
	default:
		// Some Salesforce record IDs begin with a numeric key prefix containing
		// E (for example CronTrigger's 08e prefix). Those bare IDs otherwise
		// look like scientific notation to big.Rat and are parsed as decimals.
		// Keep the known Salesforce ID shape ahead of decimal parsing.
		if looksLikeSalesforceIDLiteral(text) {
			return storage.IDValue(storage.ID(text)), storage.Value{}, false, nil
		}
		if looksDecimalLiteral(text) {
			if _, ok := new(big.Rat).SetString(text); ok {
				return storage.DecimalValue(text), storage.Value{}, false, nil
			}
		}
		if t, ok := parseISODateTime(text); ok {
			return storage.DateTimeValue(t.UTC().Format(time.RFC3339)), storage.Value{}, false, nil
		}
		if t, ok := parseISODate(text); ok {
			return storage.DateValue(t.Format("2006-01-02")), storage.Value{}, false, nil
		}
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return storage.IDValue(storage.ID(text)), storage.Value{}, false, nil
		}
		return storage.IntegerValue(value), storage.Value{}, false, nil
	}
}

func looksLikeSalesforceIDLiteral(text string) bool {
	if len(text) != 15 && len(text) != 18 || !strings.ContainsAny(text, "eE") {
		return false
	}
	if err := storage.ValidateID(storage.ID(text)); err != nil {
		return false
	}
	// The collision is possible only when the first two key-prefix
	// characters are digits and the third is E/e; any other alphabetic
	// Salesforce prefix already fails big.Rat's decimal parser.
	return isASCIIDigit(text[0]) && isASCIIDigit(text[1]) && (text[2] == 'e' || text[2] == 'E')
}

func unescapeSOQLStringLiteral(inner string, allowLikeEscapes bool) (string, error) {
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		if ch == '\'' && i+1 < len(inner) && inner[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		if ch == '\\' {
			if i+1 >= len(inner) {
				return "", fmt.Errorf("soql: backslash at end of string literal")
			}
			i++
			switch inner[i] {
			case 'n', 'N':
				b.WriteByte('\n')
			case 'r', 'R':
				b.WriteByte('\r')
			case 't', 'T':
				b.WriteByte('\t')
			case 'b', 'B':
				b.WriteByte('\a')
			case 'f', 'F':
				b.WriteByte('\f')
			case '\'', '"':
				b.WriteByte(inner[i])
			case '\\':
				if allowLikeEscapes {
					// Keep the escape marker so LIKE can distinguish a literal
					// backslash from a wildcard escape after decoding.
					b.WriteString(`\\`)
				} else {
					b.WriteByte('\\')
				}
			case '_', '%':
				if !allowLikeEscapes {
					return "", fmt.Errorf("soql: \\%c is valid only in a LIKE expression", inner[i])
				}
				b.WriteByte('\\')
				b.WriteByte(inner[i])
			case 'u':
				if i+4 >= len(inner) {
					return "", fmt.Errorf("soql: Unicode escape must contain four hexadecimal digits")
				}
				value, err := strconv.ParseUint(inner[i+1:i+5], 16, 16)
				if err != nil || value >= 0xD800 && value <= 0xDFFF {
					return "", fmt.Errorf("soql: invalid Unicode escape \\u%s", inner[i+1:i+5])
				}
				b.WriteRune(rune(value))
				i += 4
			default:
				return "", fmt.Errorf("soql: invalid backslash escape \\%c", inner[i])
			}
			continue
		}
		b.WriteByte(ch)
	}
	return b.String(), nil
}
func looksDecimalLiteral(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if trimmed[0] == '-' || trimmed[0] == '+' {
		trimmed = trimmed[1:]
	}
	if trimmed == "" {
		return false
	}
	return strings.ContainsAny(trimmed, ".eE")
}
func dateLiteral(text string, now time.Time) (storage.Value, storage.Value, bool) {
	return dateLiteralWithFiscalYearStartMonth(text, now, 1)
}

func dateLiteralWithFiscalYearStartMonth(text string, now time.Time, fiscalYearStartMonth int) (storage.Value, storage.Value, bool) {
	fiscalYearStartMonth = normalizeFiscalYearStartMonth(fiscalYearStartMonth)
	today := dateOnly(now)
	upper := strings.ToUpper(text)
	switch upper {
	case "TODAY":
		return dateRange(today, today.AddDate(0, 0, 1))
	case "YESTERDAY":
		return dateRange(today.AddDate(0, 0, -1), today)
	case "TOMORROW":
		return dateRange(today.AddDate(0, 0, 1), today.AddDate(0, 0, 2))
	case "THIS_MONTH":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, start.AddDate(0, 1, 0))
	case "LAST_MONTH":
		thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return dateRange(thisMonth.AddDate(0, -1, 0), thisMonth)
	case "NEXT_MONTH":
		nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return dateRange(nextMonth, nextMonth.AddDate(0, 1, 0))
	case "THIS_YEAR":
		start := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, start.AddDate(1, 0, 0))
	case "LAST_YEAR":
		thisYear := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(thisYear.AddDate(-1, 0, 0), thisYear)
	case "NEXT_YEAR":
		nextYear := time.Date(today.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(nextYear, nextYear.AddDate(1, 0, 0))
	case "THIS_QUARTER":
		start := quarterStart(today)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "LAST_QUARTER":
		start := quarterStart(today).AddDate(0, -3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "NEXT_QUARTER":
		start := quarterStart(today).AddDate(0, 3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "THIS_WEEK":
		start := weekStart(today)
		return dateRange(start, start.AddDate(0, 0, 7))
	case "LAST_WEEK":
		start := weekStart(today).AddDate(0, 0, -7)
		return dateRange(start, start.AddDate(0, 0, 7))
	case "NEXT_WEEK":
		start := weekStart(today).AddDate(0, 0, 7)
		return dateRange(start, start.AddDate(0, 0, 7))
	case "LAST_90_DAYS":
		return dateRange(today.AddDate(0, 0, -90), today.AddDate(0, 0, 1))
	case "NEXT_90_DAYS":
		return dateRange(today.AddDate(0, 0, 1), today.AddDate(0, 0, 91))
	case "THIS_FISCAL_QUARTER":
		start := fiscalQuarterStart(today, fiscalYearStartMonth)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "LAST_FISCAL_QUARTER":
		start := fiscalQuarterStart(today, fiscalYearStartMonth).AddDate(0, -3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "NEXT_FISCAL_QUARTER":
		start := fiscalQuarterStart(today, fiscalYearStartMonth).AddDate(0, 3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "THIS_FISCAL_YEAR":
		start := fiscalYearStart(today, fiscalYearStartMonth)
		return dateRange(start, start.AddDate(1, 0, 0))
	case "LAST_FISCAL_YEAR":
		start := fiscalYearStart(today, fiscalYearStartMonth).AddDate(-1, 0, 0)
		return dateRange(start, start.AddDate(1, 0, 0))
	case "NEXT_FISCAL_YEAR":
		start := fiscalYearStart(today, fiscalYearStartMonth).AddDate(1, 0, 0)
		return dateRange(start, start.AddDate(1, 0, 0))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_DAYS:"); ok {
		return dateRange(today.AddDate(0, 0, -n), today.AddDate(0, 0, 1))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_DAYS:"); ok {
		return dateRange(today.AddDate(0, 0, 1), today.AddDate(0, 0, n+1))
	}
	if n, ok := literalNumberSuffix(upper, "N_DAYS_AGO:"); ok {
		start := today.AddDate(0, 0, -n)
		return dateRange(start, start.AddDate(0, 0, 1))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_WEEKS:"); ok {
		// LAST_N_WEEKS contains the previous n complete weeks and excludes
		// the current week. The upper bound is the current week start.
		end := weekStart(today)
		start := end.AddDate(0, 0, -7*n)
		return dateRange(start, end)
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_WEEKS:"); ok {
		start := weekStart(today).AddDate(0, 0, 7)
		return dateRange(start, start.AddDate(0, 0, 7*n))
	}
	if n, ok := literalNumberSuffix(upper, "N_WEEKS_AGO:"); ok {
		start := weekStart(today).AddDate(0, 0, -7*n)
		return dateRange(start, start.AddDate(0, 0, 7))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_MONTHS:"); ok {
		thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		start := thisMonth.AddDate(0, -n, 0)
		return dateRange(start, thisMonth)
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_MONTHS:"); ok {
		nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return dateRange(nextMonth, nextMonth.AddDate(0, n, 0))
	}
	if n, ok := literalNumberSuffix(upper, "N_MONTHS_AGO:"); ok {
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -n, 0)
		return dateRange(start, start.AddDate(0, 1, 0))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_QUARTERS:"); ok {
		start := quarterStart(today).AddDate(0, -3*n, 0)
		return dateRange(start, quarterStart(today))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_QUARTERS:"); ok {
		start := quarterStart(today).AddDate(0, 3, 0)
		return dateRange(start, start.AddDate(0, 3*n, 0))
	}
	if n, ok := literalNumberSuffix(upper, "N_QUARTERS_AGO:"); ok {
		start := quarterStart(today).AddDate(0, -3*n, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_YEARS:"); ok {
		start := time.Date(today.Year()-n, 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_YEARS:"); ok {
		start := time.Date(today.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, time.Date(today.Year()+n+1, 1, 1, 0, 0, 0, 0, time.UTC))
	}
	if n, ok := literalNumberSuffix(upper, "N_YEARS_AGO:"); ok {
		start := time.Date(today.Year()-n, 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, start.AddDate(1, 0, 0))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_FISCAL_QUARTERS:"); ok {
		start := fiscalQuarterStart(today, fiscalYearStartMonth).AddDate(0, -3*n, 0)
		return dateRange(start, fiscalQuarterStart(today, fiscalYearStartMonth))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_FISCAL_QUARTERS:"); ok {
		start := fiscalQuarterStart(today, fiscalYearStartMonth).AddDate(0, 3, 0)
		return dateRange(start, start.AddDate(0, 3*n, 0))
	}
	if n, ok := literalNumberSuffix(upper, "N_FISCAL_QUARTERS_AGO:"); ok {
		start := fiscalQuarterStart(today, fiscalYearStartMonth).AddDate(0, -3*n, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_FISCAL_YEARS:"); ok {
		start := fiscalYearStart(today, fiscalYearStartMonth).AddDate(-n, 0, 0)
		return dateRange(start, fiscalYearStart(today, fiscalYearStartMonth))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_FISCAL_YEARS:"); ok {
		start := fiscalYearStart(today, fiscalYearStartMonth).AddDate(1, 0, 0)
		return dateRange(start, start.AddDate(n, 0, 0))
	}
	if n, ok := literalNumberSuffix(upper, "N_FISCAL_YEARS_AGO:"); ok {
		start := fiscalYearStart(today, fiscalYearStartMonth).AddDate(-n, 0, 0)
		return dateRange(start, start.AddDate(1, 0, 0))
	}
	return storage.Value{}, storage.Value{}, false
}
func isKnownUnsupportedDateLiteral(text string) bool {
	return false
}
func weekStart(day time.Time) time.Time {
	return day.AddDate(0, 0, -int(day.Weekday()))
}
func quarterStart(day time.Time) time.Time {
	month := time.Month(((int(day.Month()) - 1) / 3 * 3) + 1)
	return time.Date(day.Year(), month, 1, 0, 0, 0, 0, time.UTC)
}
func fiscalQuarterStart(day time.Time, fiscalYearStartMonth int) time.Time {
	start := fiscalYearStart(day, fiscalYearStartMonth)
	months := (day.Year()-start.Year())*12 + int(day.Month()) - int(start.Month())
	return start.AddDate(0, (months/3)*3, 0)
}
func fiscalYearStart(day time.Time, fiscalYearStartMonth int) time.Time {
	month := time.Month(normalizeFiscalYearStartMonth(fiscalYearStartMonth))
	start := time.Date(day.Year(), month, 1, 0, 0, 0, 0, time.UTC)
	if day.Before(start) {
		return time.Date(day.Year()-1, month, 1, 0, 0, 0, 0, time.UTC)
	}
	return start
}
func fiscalMonthNumber(month, fiscalYearStartMonth int) int {
	start := normalizeFiscalYearStartMonth(fiscalYearStartMonth)
	return ((month - start + 12) % 12) + 1
}
func fiscalQuarterNumber(month, fiscalYearStartMonth int) int {
	return ((fiscalMonthNumber(month, fiscalYearStartMonth) - 1) / 3) + 1
}
func fiscalYearNumber(day time.Time, fiscalYearStartMonth int, useStartYear bool) int {
	startMonth := normalizeFiscalYearStartMonth(fiscalYearStartMonth)
	startYear := day.Year()
	if int(day.Month()) < startMonth {
		startYear--
	}
	if useStartYear || startMonth == 1 {
		return startYear
	}
	return startYear + 1
}
func normalizeFiscalYearStartMonth(month int) int {
	if month < 1 || month > 12 {
		return 1
	}
	return month
}
func literalNumberSuffix(text, prefix string) (int, bool) {
	if !strings.HasPrefix(text, prefix) {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(text, prefix))
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}
func dateOnly(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
func dateRange(start, end time.Time) (storage.Value, storage.Value, bool) {
	return storage.DateValue(start.Format("2006-01-02")), storage.DateValue(end.Format("2006-01-02")), true
}
func parseISODate(text string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
func parseISODateTime(text string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z0700"} {
		if t, err := time.Parse(layout, normalizeDateTime(text)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func looksLikeISODateTime(text string) bool {
	if len(text) < 19 || text[4] != '-' || text[7] != '-' || text[10] != 'T' || text[13] != ':' || text[16] != ':' {
		return false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18} {
		if !isASCIIDigit(text[index]) {
			return false
		}
	}
	return true
}

func validateISODateTimeToken(query string, start int, text string) error {
	message := "Invalid datetime: " + text
	lexerMessage := ""
	lexerOffset := 0
	if len(text) > 19 && text[19] == ',' {
		message = "unexpected token: ':'"
		lexerMessage = "no viable alternative at character ','"
		lexerOffset = 19
	} else if len(text) > 20 && text[19] == '.' && !isASCIIDigit(text[20]) {
		message = "unexpected token: ':'"
		lexerMessage = fmt.Sprintf("required (...)+ loop did not match anything at character '%c'", text[20])
		lexerOffset = 20
	} else if _, valid := parseISODateTime(text); valid {
		return nil
	} else if offset := strings.LastIndexAny(text[19:], "+-"); offset >= 0 {
		offset += 19
		if len(text) > offset+2 && text[offset+1] == '2' && text[offset+2] >= '4' && text[offset+2] <= '9' {
			lexerMessage = fmt.Sprintf("mismatched character '%c' expecting set '0'..'3'", text[offset+2])
			lexerOffset = offset + 2
		}
	}
	err := &QueryError{Message: message, CompileSyntax: true, Err: fmt.Errorf("soql: invalid datetime literal %q", text)}
	if lexerMessage != "" {
		prefix := query[:start+lexerOffset]
		line := strings.Count(prefix, "\n") + 1
		column := len(prefix) - strings.LastIndexByte(prefix, '\n') - 1
		err.DateTimeLexerMessage = fmt.Sprintf("line %d:%d %s", line, column, lexerMessage)
	}
	return err
}
