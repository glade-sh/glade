package soql

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

type token struct {
	text string
}

func lex(input string) ([]token, error) {
	var out []token
	for i := 0; i < len(input); {
		switch {
		case input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r':
			i++
		case input[i] == '\'':
			start := i
			i++
			for i < len(input) {
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
			return nil, fmt.Errorf("soql: unterminated string literal")
		case strings.ContainsRune(",=!*()<>", rune(input[i])):
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
			for i < len(input) && !strings.ContainsRune(" \n\t\r,=!*()<>", rune(input[i])) {
				i++
			}
			out = append(out, token{text: input[start:i]})
		}
	next:
	}
	out = append(out, token{text: ""})
	return out, nil
}

type parser struct {
	tokens []token
	pos    int
	now    time.Time
}

func (p *parser) parseQuery() (Query, error) {
	if !p.matchWord("SELECT") {
		return Query{}, p.errorf("expected SELECT")
	}
	fields, childQueries, typeofs, err := p.parseFields()
	if err != nil {
		return Query{}, err
	}
	if !p.matchWord("FROM") {
		return Query{}, p.errorf("expected FROM")
	}
	object, err := p.parseName()
	if err != nil {
		return Query{}, err
	}
	relationshipAliases, err := p.parseRelationshipAliases(object)
	if err != nil {
		return Query{}, err
	}
	if len(relationshipAliases) != 0 {
		fields = rewriteRelationshipAliasNames(fields, relationshipAliases)
	}
	aggregates, err := aggregateSpecs(fields)
	if err != nil {
		return Query{}, err
	}
	q := Query{Fields: fields, ChildQueries: childQueries, Typeofs: typeofs, Object: object, Count: len(fields) == 1 && strings.EqualFold(fields[0], "COUNT()"), Aggregates: aggregates}
	for p.peek().text != "" && p.peek().text != ")" {
		switch {
		case p.matchWord("WHERE"):
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			if len(relationshipAliases) != 0 {
				rewriteRelationshipAliasCondition(&condition, relationshipAliases)
			}
			q.Where = &condition
		case p.matchWord("GROUP"):
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after GROUP")
			}
			groupMode := ""
			if p.matchWord("ROLLUP") || p.matchWord("CUBE") {
				groupMode = strings.ToUpper(p.tokens[p.pos-1].text)
				if !p.match("(") {
					return Query{}, p.errorf("expected ( after GROUP BY %s", groupMode)
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
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			condition = rewriteHavingAggregates(condition, &q)
			q.Having = &condition
		case p.matchWord("ORDER"):
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
			limit, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Limit = limit
			q.HasLimit = true
		case p.matchWord("OFFSET"):
			offset, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Offset = offset
		case p.matchWord("FOR"):
			switch {
			case p.matchWord("UPDATE"):
				q.ForUpdate = true
			case p.matchWord("VIEW"):
				q.ForView = true
			default:
				return Query{}, p.errorf("expected UPDATE or VIEW after FOR")
			}
		case p.matchWord("ALL"):
			if !p.matchWord("ROWS") {
				return Query{}, p.errorf("expected ROWS after ALL")
			}
			q.AllRows = true
		case p.matchWord("WITH"):
			mode, err := p.parseSecurityMode()
			if err != nil {
				return Query{}, err
			}
			q.SecurityMode = mode
		default:
			return Query{}, unsupportedSOQLErrorf("unsupported SOQL token %q", p.peek().text)
		}
	}
	if err := validateAggregateQuery(q); err != nil {
		return Query{}, err
	}
	return q, nil
}
func (p *parser) parseRelationshipAliases(rootObject string) (map[string]string, error) {
	aliases := map[string]string(nil)
	for p.match(",") {
		path, err := p.parseName()
		if err != nil {
			return nil, err
		}
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
	case "WHERE", "GROUP", "HAVING", "ORDER", "LIMIT", "OFFSET", "FOR", "ALL", "WITH":
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
			continue
		}
		if p.peek().text == "" || p.peek().text == ")" || strings.EqualFold(p.peek().text, "FROM") {
			return fields, childQueries, typeofs, nil
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
				arg, err := p.parseName()
				if err != nil {
					return nil, nil, nil, err
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
				field += " " + alias
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
		field, err := p.parseName()
		if err != nil {
			return nil, err
		}
		if field == "" {
			return nil, p.errorf("expected ORDER BY field")
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
				return nil, p.errorf("expected FIRST or LAST after NULLS")
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
	args := strings.Split(argsText, ",")
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
		if args[i] == "" {
			return selectFieldExpression{}, false
		}
	}
	alias := ""
	if len(parts) == 2 {
		alias = parts[1]
	}
	return selectFieldExpression{Func: fn, Args: args, Alias: alias, Raw: raw}, true
}
func validateSelectFieldExpression(org storage.OrgState, definition storage.ObjectDefinition, expr selectFieldExpression, mode string) error {
	if len(expr.Args) != 1 {
		return unsupportedSOQLErrorf("%s currently supports one field argument", expr.Func)
	}
	return validateFieldReference(org, definition, selectFunctionFieldArg(expr.Args[0]), mode)
}
func selectFieldExpressionValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, expr selectFieldExpression) (storage.Value, bool) {
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
	case "CALENDAR_MONTH", "FISCAL_MONTH":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.Month())), true
			}
		}
	case "CALENDAR_QUARTER", "FISCAL_QUARTER":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64((int(parsed.Month())-1)/3 + 1)), true
			}
		}
	case "CALENDAR_YEAR", "FISCAL_YEAR":
		if text, ok := storageValueDateText(value); ok {
			if parsed, err := time.Parse("2006-01-02", text[:10]); err == nil {
				return storage.IntegerValue(int64(parsed.Year())), true
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
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
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
	if strings.HasSuffix(text, "Z") || strings.Contains(text, "+") {
		return text
	}
	return text + "Z"
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
func validateAggregateQuery(query Query) error {
	if len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 {
		if query.Having != nil {
			return fmt.Errorf("soql: GROUP BY and HAVING require aggregate fields")
		}
		return nil
	}
	for _, field := range query.Fields {
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
		if !containsName(query.GroupBy, groupingComparableField(field)) {
			return fmt.Errorf("soql: field %s must be grouped or aggregated", field)
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
		if aggregate.Alias != "" {
			out[aggregate.Alias] = aggregate.Alias
		}
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
	return Condition{And: append([]Condition{left}, ands...)}, nil
}
func (p *parser) parsePrimaryCondition() (Condition, error) {
	if p.matchWord("NOT") {
		cond, err := p.parsePrimaryCondition()
		if err != nil {
			return Condition{}, err
		}
		cond.Not = true
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
	if op == "IN" || op == "NOT IN" {
		values, subquery, err := p.parseInOperand()
		if err != nil {
			return Condition{}, err
		}
		return Condition{Field: field, Op: op, Values: values, Subquery: subquery}, nil
	}
	parenthesizedValue := p.match("(")
	valueToken := p.advance().text
	if valueToken == "" {
		return Condition{}, p.errorf("expected WHERE value")
	}
	valueToken = p.signedLiteralToken(valueToken)
	value, value2, isRange, err := literalAt(valueToken, p.now)
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
			tok = p.signedLiteralToken(tok)
			nextValue, _, nextRange, err := literalAt(tok, p.now)
			if err != nil {
				return Condition{}, err
			}
			values = append(values, nextValue)
			ranges = append(ranges, nextRange)
		}
		if !p.match(")") {
			return Condition{}, p.errorf("expected ) after WHERE value")
		}
		if len(values) > 1 && (op == "LIKE" || op == "NOT LIKE") {
			conditions := make([]Condition, 0, len(values))
			for i, item := range values {
				conditions = append(conditions, Condition{Field: field, Op: op, Value: item, Range: ranges[i]})
			}
			if op == "NOT LIKE" {
				return Condition{And: conditions}, nil
			}
			return Condition{Or: conditions}, nil
		}
	}
	return Condition{Field: field, Op: op, Value: value, Value2: value2, Range: isRange}, nil
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
func (p *parser) signedLiteralToken(tok string) string {
	if tok != "-" && tok != "+" {
		return tok
	}
	next := p.peek().text
	if next == "" || strings.ContainsRune(",)(", rune(next[0])) {
		return tok
	}
	p.advance()
	return tok + next
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
func (p *parser) parseInOperand() ([]storage.Value, *Query, error) {
	if !p.match("(") {
		tok := p.advance().text
		if tok == "" {
			return nil, nil, p.errorf("expected value after IN")
		}
		tok = p.signedLiteralToken(tok)
		value, _, isRange, err := literalAt(tok, p.now)
		if err != nil {
			return nil, nil, err
		}
		if isRange {
			return nil, nil, unsupportedSOQLErrorf("date range literal %s is not supported in IN lists", tok)
		}
		return []storage.Value{value}, nil, nil
	}
	if strings.EqualFold(p.peek().text, "SELECT") {
		query, err := p.parseQuery()
		if err != nil {
			return nil, nil, err
		}
		if len(query.Fields) != 1 {
			return nil, nil, p.errorf("semi-join subquery must select one field")
		}
		if !p.match(")") {
			return nil, nil, p.errorf("expected ) after semi-join subquery")
		}
		return nil, &query, nil
	}
	var values []storage.Value
	if p.match(")") {
		return values, nil, nil
	}
	for {
		tok := p.advance().text
		if tok == "" {
			return nil, nil, p.errorf("expected value in IN list")
		}
		tok = p.signedLiteralToken(tok)
		value, _, isRange, err := literalAt(tok, p.now)
		if err != nil {
			return nil, nil, err
		}
		if isRange {
			return nil, nil, unsupportedSOQLErrorf("date range literal %s is not supported in IN lists", tok)
		}
		values = append(values, value)
		if p.match(")") {
			break
		}
		if !p.match(",") {
			return nil, nil, p.errorf("expected , or ) in IN list")
		}
	}
	return values, nil, nil
}
func (p *parser) parseName() (string, error) {
	name := p.advance().text
	if name == "" {
		return "", p.errorf("expected name")
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
	if start, end, ok := dateLiteral(text, now); ok {
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
		return storage.StringValue(strings.ReplaceAll(inner, "''", "'")), storage.Value{}, false, nil
	default:
		if looksDecimalLiteral(text) {
			if _, ok := new(big.Rat).SetString(text); ok {
				return storage.DecimalValue(text), storage.Value{}, false, nil
			}
		}
		if t, ok := parseISODateTime(text); ok {
			return storage.DateTimeValue(t.Format(time.RFC3339)), storage.Value{}, false, nil
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
	case "LAST_90_DAYS":
		return dateRange(today.AddDate(0, 0, -90), today.AddDate(0, 0, 1))
	case "NEXT_90_DAYS":
		return dateRange(today, today.AddDate(0, 0, 91))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_DAYS:"); ok {
		return dateRange(today.AddDate(0, 0, -n), today.AddDate(0, 0, 1))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_DAYS:"); ok {
		return dateRange(today, today.AddDate(0, 0, n+1))
	}
	if n, ok := literalNumberSuffix(upper, "N_DAYS_AGO:"); ok {
		start := today.AddDate(0, 0, -n)
		return dateRange(start, start.AddDate(0, 0, 1))
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
	return storage.Value{}, storage.Value{}, false
}
func isKnownUnsupportedDateLiteral(text string) bool {
	upper := strings.ToUpper(text)
	switch upper {
	case "THIS_WEEK", "LAST_WEEK", "NEXT_WEEK":
		return true
	default:
		return strings.HasPrefix(upper, "LAST_N_WEEKS:") || strings.HasPrefix(upper, "NEXT_N_WEEKS:")
	}
}
func quarterStart(day time.Time) time.Time {
	month := time.Month(((int(day.Month()) - 1) / 3 * 3) + 1)
	return time.Date(day.Year(), month, 1, 0, 0, 0, 0, time.UTC)
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
	t, err := time.Parse(time.RFC3339, normalizeDateTime(text))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
