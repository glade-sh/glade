package project

import (
	"encoding/xml"
	"strings"
)

// One file can supply several feature predicates. Each consumes the same
// immutable stripped source, string mask, query list and metadata tree.
type setupEvidenceScan struct {
	raw         string
	metadata    bool
	prepared    bool
	source      string
	code        string
	haveQueries bool
	queries     []string
	triedXML    bool
	validXML    bool
	root        setupMetadataNode
	foldedReady bool
	ascii       bool
	folded      string
}

// A typed property, getter or constructor assignment must contain the actual
// field text. Retain the regexp path on non-ASCII input for Unicode folding.
func (s *setupEvidenceScan) mayContain(terms ...string) bool {
	if !s.foldedReady {
		s.foldedReady = true
		s.ascii = true
		for i := 0; i < len(s.raw); i++ {
			if s.raw[i] >= 128 {
				s.ascii = false
				break
			}
		}
		if s.ascii {
			s.folded = strings.ToLower(s.raw)
		}
	}
	if !s.ascii {
		return true
	}
	for _, term := range terms {
		if strings.Contains(s.folded, term) {
			return true
		}
	}
	return false
}

func (s *setupEvidenceScan) text() (string, string) {
	if !s.prepared {
		s.source = stripApexComments(s.raw)
		s.code = maskSetupStrings(s.source)
		s.prepared = true
	}
	return s.source, s.code
}

func (s *setupEvidenceScan) executedQueries() []string {
	if !s.haveQueries {
		source, code := s.text()
		s.queries = setupExecutedQueries(source, code)
		s.haveQueries = true
	}
	return s.queries
}

func (s *setupEvidenceScan) metadataUses(feature string) bool {
	if !s.triedXML {
		s.validXML = xml.Unmarshal([]byte(s.raw), &s.root) == nil
		s.triedXML = true
	}
	return s.validXML && setupMetadataNodeUsesFeature(s.root, feature)
}

// Only an actual field identifier, currency object or currency function
// can make the AST predicate true. Ambiguous text goes through the parser.
var setupQueryWitnesses = func() map[string]map[string]bool {
	result := map[string]map[string]bool{}
	for feature, objects := range setupFieldMetadata {
		terms := map[string]bool{}
		for _, fields := range objects {
			for _, field := range fields {
				terms[strings.ToLower(field)] = true
			}
		}
		result[feature] = terms
	}
	return result
}()

func queryCouldUseSetupFeature(query, feature string, custom map[string]bool) bool {
	if feature != "MultiCurrency" && feature != "PersonAccounts" {
		return true
	}
	terms, ok := setupQueryWitnesses[feature]
	if !ok {
		return true
	}
	// Check the original bytes. Unicode lowercase maps some non-ASCII input
	// to ASCII, while the parser and its field predicates use their own rules.
	for i := 0; i < len(query); i++ {
		if query[i] >= 0x80 {
			return true
		}
	}
	lower := strings.ToLower(query)
	start := -1
	for i := 0; i <= len(lower); i++ {
		if i < len(lower) {
			c := lower[i]
			if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' {
				if start < 0 {
					start = i
				}
				continue
			}
		}
		if start < 0 {
			continue
		}
		token := lower[start:i]
		if terms[token] || feature == "PersonAccounts" && custom[token] || feature == "MultiCurrency" && (currencyTypeRE.get().MatchString(token) || token == "convertcurrency") {
			return true
		}
		start = -1
	}
	// The AST strips the first qualifier and looks up the entire remaining
	// source range. A project key can contain a second dot, spacing within
	// that path, or a dollar sign allowed by the embedded Apex identifier rule.
	// All other ASCII field names are individual tokens already checked above.
	if feature == "PersonAccounts" && len(custom) > 0 && strings.ContainsAny(lower, ".$") {
		for name, present := range custom {
			if present && strings.Contains(lower, name) {
				return true
			}
		}
	}
	return false
}
