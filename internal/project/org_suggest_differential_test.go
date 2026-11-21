package project

import (
	"regexp"
	"strings"
	"testing"

	apexast "github.com/glade-sh/apex-parser"
)

func TestSetupEvidenceScanMatchesOriginal(t *testing.T) {
	sources := []struct {
		text     string
		metadata bool
	}{
		{"class Sample { void m(){ Account a = new Account(IsPersonAccount=true); } }", false},
		{"class Sample { void m(Opportunity o){ o.CurrencyIsoCode='USD'; } }", false},
		{"class Sample { void m(){ Database.query('SELECT a.Color__pc FROM Account a'); } }", false},
		{"class Sample { void m(){ List<Network> n = [SELECT Id FROM Network]; } }", false},
		{"class Sample { void m(){ User u = new User(IsPortalEnabled=true); } }", false},
		{"class Sample { void m(){ String s = Site.getBaseUrl(); } }", false},
		{"class Sample { void m(){ Opportunity o = new Opportunity(Territory2Id=null); } }", false},
		{"class Sample { String s = 'CurrencyIsoCode IsPersonAccount Site'; /* Network.getNetworkId() */ }", false},
		{"class Sample { void m(){ Database.query('SELECT Id FROM Account WHERE Name = 'K''); } }", false},
		{"\xff SELECT Id FROM Account", false},
		{"<CustomField><fullName>Account.PersonEmail</fullName></CustomField>", true},
		{"<broken>", true},
		{"<CustomField><fullName>Account.CurrencyIsoCode</fullName></CustomField>", true},
	}
	for _, input := range sources {
		scan := setupEvidenceScan{raw: input.text, metadata: input.metadata}
		for _, custom := range []map[string]bool{nil, {"color__pc": true}, {"other__pc": true}} {
			for _, rule := range featureEvidenceRules {
				rule.customPersonFields = custom
				got, want := scan.matches(rule), originalMatchesSetupFeature(rule, input.text, input.metadata)
				if got != want {
					t.Errorf("feature %s source %q metadata %v: got %v want %v", rule.feature, input.text, input.metadata, got, want)
				}
			}
		}
	}
}

func originalMatchesSetupFeature(rule featureEvidence, source string, metadata bool) bool {
	if rule.feature == "PersonAccounts" || rule.feature == "MultiCurrency" {
		return originalMatchesPlatformFieldFeature(rule, source, metadata)
	}
	if metadata {
		return setupMetadataUsesFeature(source, rule.feature)
	}
	source = stripApexComments(source)
	code := maskSetupStrings(source)
	if rule.feature == "Sites" {
		// Match an identifier, member access, method identifier and call token.
		// Comments and literal contents cannot supply any of these tokens.
		tokens := setupCodeTokenRE.FindAllString(code, -1)
		for i := 0; i+3 < len(tokens); i++ {
			if tokens[i] == "Site" && tokens[i+1] == "." && tokens[i+3] == "(" && rule.pattern.MatchString("Site."+tokens[i+2]) {
				return true
			}
		}
		return false
	}
	for _, loc := range schemaImportRE.FindAllStringSubmatchIndex(source, -1) {
		if code[loc[0]] == source[loc[0]] && setupMetadataReference(source[loc[2]:loc[3]], rule.feature) {
			return true
		}
	}
	// NetworksEnabled needs no pattern here: only queries and schema references
	// of the objects count, and the Network class calls are Communities evidence.
	if rule.feature == "Communities" {
		if networkCallRE.MatchString(code) || typedSetupFieldEvidence(source, code, "User", `IsPortalEnabled\b|UserType\s*={1,2}\s*'(?:CustomerSuccess|PowerCustomerSuccess|PowerPartner|CspLitePortal)'|(?:get|put)\s*\(\s*'IsPortalEnabled'`) || setupConstructorEvidence(source, code, "User", portalAssignmentRE) {
			return true
		}
	} else if rule.feature == "NetworksEnabled" && (networkSchemaTokenRE.MatchString(code) || setupMatchOutsideString(networkSchemaLookupRE, source, code)) {
		return true
	} else if rule.feature == "Territory2" && (territoryUseRE.MatchString(code) || territoryNewRE.MatchString(code) || setupMatchOutsideString(territorySchemaRE, source, code) || typedSetupFieldEvidence(source, code, "Opportunity", `Territory2Id\b|(?:get|put)\s*\(\s*'Territory2Id'`) || setupConstructorEvidence(source, code, "Opportunity", territoryAssignmentRE)) {
		return true
	}
	for _, query := range setupExecutedQueries(source, code) {
		root := fromObjectRE.FindStringSubmatch(query)
		if len(root) != 2 {
			continue
		}
		queryCode := maskSetupStrings(query)
		if rule.feature == "Territory2" {
			if territoryTypeRE.MatchString(root[1]) || (strings.EqualFold(root[1], "Opportunity") && territoryFieldRE.MatchString(queryCode)) {
				return true
			}
		} else if rule.feature == "NetworksEnabled" {
			if networkObjectRE.MatchString(root[1]) {
				return true
			}
		} else if strings.EqualFold(root[1], "User") && (portalFieldRE.MatchString(queryCode) || setupMatchOutsideString(externalUserTypeRE, query, queryCode)) {
			return true
		}
	}
	return false
}

func originalMatchesPlatformFieldFeature(rule featureEvidence, source string, metadata bool) bool {
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
		if originalSetupQueryUsesPlatformFeature(query, rule.feature, rule.customPersonFields) {
			return true
		}
	}
	return false
}

// The standalone parser exposes field identifiers separately from result aliases,
// bind expressions and literal values. Only field nodes consult standard or
// project field metadata.
func originalSetupQueryUsesPlatformFeature(query, feature string, customPersonFields map[string]bool) bool {
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
