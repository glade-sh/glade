package project

import (
	"regexp"
	"strings"

	apexast "github.com/glade-sh/apex-parser"
)

const personAccountFieldPattern = `(?:IsPersonAccount|PersonContactId|Person(?:Email|MobilePhone|HomePhone|OtherPhone|Birthdate|Title|Department|LeadSource|AssistantName|AssistantPhone|HasOptedOutOfEmail|DoNotCall|MailingStreet|MailingCity|MailingState|MailingPostalCode|MailingCountry|MailingStateCode|MailingCountryCode|OtherStreet|OtherCity|OtherState|OtherPostalCode|OtherCountry)|[A-Za-z][A-Za-z0-9_]*__pc)`

var (
	personAccountFieldRE = regexp.MustCompile(`(?i)^` + personAccountFieldPattern + `$`)
	currencyTypeRE       = regexp.MustCompile(`(?i)^(?:CurrencyType|DatedConversionRate)$`)
	currencyObjectRE     = regexp.MustCompile(`(?i)^(?:Account|Contact|Lead|Opportunity|Case|Campaign|Contract|Order|OrderItem|Quote|QuoteLineItem|OpportunityLineItem|PricebookEntry|Task|Event|SObject|[A-Za-z][A-Za-z0-9_]*__c)$`)
	setupIdentifierRE    = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_]*\b`)
	setupDatabaseQueryRE = regexp.MustCompile(`(?i)\bDatabase\s*\.\s*(?:query|queryWithBinds|getQueryLocator|getQueryLocatorWithBinds|countQuery|countQueryWithBinds)\s*\(\s*('(?:\\.|[^'\\])*')`)
)

// Labels and local declarations are not field usage. Require a typed platform
// receiver, a schema reference, a constructor field, or an executed query.
func matchesPlatformFieldFeature(rule featureEvidence, source string, metadata bool) bool {
	if metadata {
		return setupMetadataUsesFeature(source, rule.feature)
	}
	source = stripApexComments(source)
	code := maskSetupStrings(source)
	for _, loc := range schemaImportRE.FindAllStringSubmatchIndex(source, -1) {
		if code[loc[0]] == source[loc[0]] && setupMetadataReference(source[loc[2]:loc[3]], rule.feature) {
			return true
		}
	}
	objects := []string{"Account", "Contact"}
	field := personAccountFieldPattern
	if rule.feature == "MultiCurrency" {
		field = `CurrencyIsoCode`
		objects = nil
		seen := map[string]bool{}
		// Include type names in schema lookups, which are string arguments.
		for _, object := range setupIdentifierRE.FindAllString(source, -1) {
			key := strings.ToLower(object)
			if currencyObjectRE.MatchString(object) && !seen[key] {
				objects = append(objects, object)
				seen[key] = true
			}
		}
	}
	for _, object := range objects {
		objectField := field
		if rule.feature == "PersonAccounts" && strings.EqualFold(object, "Contact") {
			objectField = `IsPersonAccount`
		}
		fields := objectField + `\b|(?:get|put)\s*\(\s*'` + objectField + `'`
		assignment := regexp.MustCompile(`(?i)\b` + objectField + `\s*=`)
		if typedSetupFieldEvidence(source, code, regexp.QuoteMeta(object), fields) || setupConstructorEvidence(source, code, regexp.QuoteMeta(object), assignment) {
			return true
		}
	}
	for _, query := range setupExecutedQueries(source, code) {
		if setupQueryUsesPlatformFeature(query, rule.feature, rule.customPersonFields) {
			return true
		}
	}
	return false
}

// The standalone parser exposes field identifiers separately from result aliases,
// bind expressions and literal values. Only field nodes consult standard or
// project field metadata.
func setupQueryUsesPlatformFeature(query, feature string, customPersonFields map[string]bool) bool {
	query = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(query), "["), "]")
	source := "class SetupQuery { Object rows = [" + query + "]; }"
	parsed := apexast.ParseSourceAST("SetupQuery.cls", source)
	if len(parsed.Diagnostics) != 0 {
		return false
	}
	text := func(node apexast.ASTNode) string { return source[node.Range.Start.Offset:node.Range.End.Offset] }
	var walk func(apexast.ASTNode, string, string, map[string]apexast.ASTNode, bool) bool
	walk = func(node apexast.ASTNode, object, qualifier string, aliases map[string]apexast.ASTNode, resultField bool) bool {
		switch node.Kind {
		case "bound_apex_expression", "string_literal":
			return false
		case "soql_query_body":
			object, qualifier = "", ""
			aliases = map[string]apexast.ASTNode{}
			for _, clause := range node.Children {
				if clause.Kind == "from_clause" {
					var from func(apexast.ASTNode)
					from = func(part apexast.ASTNode) {
						if part.Kind == "storage_identifier" && object == "" {
							object = text(part)
						}
						if part.Kind == "storage_alias" && len(part.Children) > 1 {
							qualifier = text(part.Children[1])
						}
						for _, child := range part.Children {
							from(child)
						}
					}
					from(clause)
				}
				if clause.Kind == "select_clause" {
					for _, part := range clause.Children {
						if part.Kind == "alias_expression" && len(part.Children) > 1 {
							aliases[strings.ToLower(text(part.Children[1]))] = part.Children[0]
						}
					}
				}
			}
			if feature == "MultiCurrency" && currencyTypeRE.MatchString(object) {
				return true
			}
		case "alias_expression":
			if len(node.Children) == 0 {
				return false
			}
			return walk(node.Children[0], object, qualifier, aliases, false)
		case "field_identifier":
			field := text(node)
			if resultField {
				if input, ok := aliases[strings.ToLower(field)]; ok {
					return walk(input, object, qualifier, nil, false)
				}
			}
			if prefix, rest, ok := strings.Cut(field, "."); ok {
				if !strings.EqualFold(prefix, object) && !strings.EqualFold(prefix, qualifier) {
					return false
				}
				field = rest
			}
			return setupFeatureFieldMetadata(feature, object, field) ||
				(feature == "PersonAccounts" && strings.EqualFold(object, "Account") && customPersonFields[strings.ToLower(field)])
		case "function_expression":
			if feature == "MultiCurrency" && len(node.Children) > 0 && strings.EqualFold(text(node.Children[0]), "convertCurrency") {
				return true
			}
		case "having_clause", "order_by_clause":
			resultField = true
		}
		for _, child := range node.Children {
			if walk(child, object, qualifier, aliases, resultField) {
				return true
			}
		}
		return false
	}
	for _, node := range parsed.Nodes {
		if walk(node, "", "", nil, false) {
			return true
		}
	}
	return false
}

func setupExecutedQueries(source, code string) []string {
	var queries []string
	for _, loc := range soqlBlockRE.FindAllStringIndex(code, -1) {
		queries = append(queries, source[loc[0]:loc[1]])
	}
	// A query-looking label is not evidence. Only literals passed to a Database
	// query entry point are inspected; values assembled at runtime stay unknown.
	joined := setupJoinedStringsRE.ReplaceAllStringFunc(source, func(s string) string {
		var parts []string
		for _, literal := range setupStringRE.FindAllString(s, -1) {
			parts = append(parts, literal[1:len(literal)-1])
		}
		return "'" + strings.Join(parts, "") + "'"
	})
	joinedCode := maskSetupStrings(joined)
	for _, loc := range setupDatabaseQueryRE.FindAllStringSubmatchIndex(joined, -1) {
		if joinedCode[loc[0]] == joined[loc[0]] {
			queries = append(queries, strings.ReplaceAll(joined[loc[2]+1:loc[3]-1], `\'`, `'`))
		}
	}
	return queries
}
