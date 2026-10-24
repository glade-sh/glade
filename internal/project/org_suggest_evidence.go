package project

import (
	"encoding/xml"
	"regexp"
	"strings"
)

var (
	setupStringRE         = regexp.MustCompile(`'(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"`)
	setupJoinedStringsRE  = regexp.MustCompile(`'(?:\\.|[^'\\])*'(?:\s*\+\s*'(?:\\.|[^'\\])*')+`)
	setupQueryRootRE      = regexp.MustCompile(`(?i)\bFROM\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s+([A-Za-z_][A-Za-z0-9_]*))?`)
	setupCodeTokenRE      = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|[^\s]`)
	networkCallRE         = regexp.MustCompile(`(?i)\bNetwork\s*\.\s*(getNetworkId|getLoginUrl|getLogoutUrl|getSelfRegUrl|forgotPassword|communitiesLanding)\s*\(`)
	portalFieldRE         = regexp.MustCompile(`(?i)\bIsPortalEnabled\b`)
	portalAssignmentRE    = regexp.MustCompile(`(?i)\bIsPortalEnabled\s*=`)
	territoryFieldRE      = regexp.MustCompile(`(?i)\bTerritory2Id\b`)
	territoryAssignmentRE = regexp.MustCompile(`(?i)\bTerritory2Id\s*=`)
	externalUserTypeRE    = regexp.MustCompile(`(?i)\bUserType\s*={1,2}\s*'(CustomerSuccess|PowerCustomerSuccess|PowerPartner|CspLitePortal)'`)
	territoryTypeRE       = regexp.MustCompile(`(?i)^(Territory2|Territory2Model|Territory2Type|UserTerritory2Association|ObjectTerritory2Association)$`)
	territoryUseRE        = regexp.MustCompile(`(?i)\b(Territory2|Territory2Model|Territory2Type|UserTerritory2Association|ObjectTerritory2Association)\b\s*(?:[>\[]|[A-Za-z_][A-Za-z0-9_]*\b|\.\s*(SObjectType|fields)\b)`)
	territoryNewRE        = regexp.MustCompile(`(?i)\bnew\s+(Territory2|Territory2Model|Territory2Type|UserTerritory2Association|ObjectTerritory2Association)\s*\(`)
	schemaImportRE        = regexp.MustCompile(`(?i)\bimport\b[^;\n]*\bfrom\s*['"]@salesforce/schema/([^'"]+)['"]`)
	territorySchemaRE     = regexp.MustCompile(`(?i)\bSchema\s*\.\s*getGlobalDescribe\s*\(\s*\)\s*\.\s*get\s*\(\s*'(Territory2|Territory2Model|Territory2Type|UserTerritory2Association|ObjectTerritory2Association)'\s*\)`)
)

// Only explicit platform contexts count as optional-feature evidence.
// Local declarations and labels can use the same spellings without org setup.
func matchesSetupFeature(rule featureEvidence, source string, metadata bool) bool {
	if rule.feature == "PersonAccounts" || rule.feature == "MultiCurrency" {
		return matchesPlatformFieldFeature(rule, source, metadata)
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
	if rule.feature == "Communities" {
		if networkCallRE.MatchString(code) || typedSetupFieldEvidence(source, code, "User", `IsPortalEnabled\b|UserType\s*={1,2}\s*'(?:CustomerSuccess|PowerCustomerSuccess|PowerPartner|CspLitePortal)'|(?:get|put)\s*\(\s*'IsPortalEnabled'`) || setupConstructorEvidence(source, code, "User", portalAssignmentRE) {
			return true
		}
	} else if territoryUseRE.MatchString(code) || territoryNewRE.MatchString(code) || setupMatchOutsideString(territorySchemaRE, source, code) || typedSetupFieldEvidence(source, code, "Opportunity", `Territory2Id\b|(?:get|put)\s*\(\s*'Territory2Id'`) || setupConstructorEvidence(source, code, "Opportunity", territoryAssignmentRE) {
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
		} else if strings.EqualFold(root[1], "Network") || strings.EqualFold(root[1], "NetworkMember") || (strings.EqualFold(root[1], "User") && (portalFieldRE.MatchString(queryCode) || setupMatchOutsideString(externalUserTypeRE, query, queryCode))) {
			return true
		}
	}
	return false
}

func maskSetupStrings(source string) string {
	return setupStringRE.ReplaceAllStringFunc(source, func(s string) string { return strings.Repeat(" ", len(s)) })
}

func setupMatchOutsideString(pattern *regexp.Regexp, source, code string) bool {
	for _, loc := range pattern.FindAllStringIndex(source, -1) {
		if code[loc[0]] == source[loc[0]] {
			return true
		}
	}
	return false
}

func setupConstructorEvidence(source, code, object string, field *regexp.Regexp) bool {
	pattern := regexp.MustCompile(`(?i)\bnew\s+` + object + `\s*\([^)]*\)`)
	for _, loc := range pattern.FindAllStringIndex(code, -1) {
		if field.MatchString(code[loc[0]:loc[1]]) || (object == "User" && setupMatchOutsideString(externalUserTypeRE, source[loc[0]:loc[1]], code[loc[0]:loc[1]])) {
			return true
		}
	}
	return false
}

func typedSetupFieldEvidence(source, code, object, fields string) bool {
	receivers := []string{object}
	variableRE := regexp.MustCompile(`(?i)\b` + object + `\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	for _, match := range variableRE.FindAllStringSubmatch(code, -1) {
		name := regexp.QuoteMeta(match[1])
		declarations := regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;(){}=]+>)?(?:\s*\[\s*\])?)\s+` + name + `\b`)
		ambiguous := false
		for _, declaration := range declarations.FindAllStringSubmatch(code, -1) {
			if strings.EqualFold(declaration[1], "return") || strings.EqualFold(declaration[1], "throw") {
				continue
			}
			if !strings.EqualFold(declaration[1], object) {
				ambiguous = true
			}
		}
		// The scanner is deliberately conservative when methods reuse a name with
		// different types; a Map key must not inherit another method's User type.
		if !ambiguous {
			receivers = append(receivers, name)
		}
	}
	spaceDot := `\s*\.\s*`
	fieldMap := `(?:` + spaceDot + `fields(?:` + spaceDot + `getMap\s*\(\s*\))?)?`
	describe := `(?:` + spaceDot + `SObjectType)?(?:` + spaceDot + `getDescribe\s*\(\s*\))?`
	dynamicSchema := `\bSchema` + spaceDot + `getGlobalDescribe\s*\(\s*\)` + spaceDot + `get\s*\(\s*'` + object + `'\s*\)` + describe + fieldMap
	receiver := `(?:\b(?:` + strings.Join(receivers, "|") + `)` + describe + fieldMap + `|\bnew\s+` + object + `\s*\([^)]*\)|` + dynamicSchema + `)` + spaceDot
	field := regexp.MustCompile(`(?i)` + receiver + `(?:` + fields + `)`)
	return setupMatchOutsideString(field, source, code)
}

type setupMetadataNode struct {
	XMLName  xml.Name
	Text     string              `xml:",chardata"`
	Children []setupMetadataNode `xml:",any"`
}

func setupMetadataUsesFeature(source, feature string) bool {
	var root setupMetadataNode
	if xml.Unmarshal([]byte(source), &root) != nil {
		return false
	}
	var visit func(setupMetadataNode, string) bool
	visit = func(node setupMetadataNode, object string) bool {
		for _, child := range node.Children {
			if child.XMLName.Local == "object" || child.XMLName.Local == "sobjectType" {
				object = strings.TrimSpace(child.Text)
			}
		}
		value := strings.TrimSpace(node.Text)
		switch node.XMLName.Local {
		case "object", "referenceTo", "sobjectType":
			if setupMetadataReference(value, feature) {
				return true
			}
		case "field":
			if !strings.Contains(value, ".") && object != "" {
				value = object + "." + value
			}
			if setupMetadataReference(value, feature) {
				return true
			}
		}
		for _, child := range node.Children {
			if visit(child, object) {
				return true
			}
		}
		return false
	}
	return visit(root, "")
}

func setupMetadataReference(value, feature string) bool {
	if feature == "Sites" {
		return false
	}
	if feature == "PersonAccounts" {
		object, field, ok := strings.Cut(value, ".")
		return ok && ((strings.EqualFold(object, "Account") && personAccountFieldRE.MatchString(field)) || (strings.EqualFold(object, "Contact") && strings.EqualFold(field, "IsPersonAccount")))
	}
	if feature == "MultiCurrency" {
		object, field, ok := strings.Cut(value, ".")
		return currencyTypeRE.MatchString(value) || (ok && currencyObjectRE.MatchString(object) && strings.EqualFold(field, "CurrencyIsoCode"))
	}
	if feature == "Territory2" {
		return territoryTypeRE.MatchString(strings.SplitN(value, ".", 2)[0]) || strings.EqualFold(value, "Opportunity.Territory2Id")
	}
	return strings.EqualFold(value, "User.IsPortalEnabled") || strings.EqualFold(value, "Network") || strings.EqualFold(value, "NetworkMember")
}
