package apexast

import "strings"

// salesforceReservedIdentifiers is the complete reserved-word table from the
// current Apex Developer Guide. Apex identifiers are case-insensitive.
var salesforceReservedIdentifiers = wordSet(`
	abstract activate and any array as asc autonomous begin bigdecimal blob
	boolean break bulk by byte case cast catch char class collect commit const
	continue currency date datetime decimal default delete desc do double else
	end enum exception exit export extends false final finally float for from
	global goto group having hint if implements import in inner insert instanceof
	int integer interface into join like limit list long loop map merge new not
	null nulls number object of on or outer override package parallel pragma
	private protected public retrieve return rollback select set short sobject
	sort static string super switch synchronized system testmethod then this
	throw time transaction trigger true try undelete update upsert using virtual
	void webservice when where while
`)

// Salesforce permits most reserved words as method names. These words are
// grammar keywords in every identifier context, including method declarations.
var salesforceAlwaysKeywords = wordSet(`
	trigger insert update upsert delete undelete merge new for select
`)

func wordSet(words string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, word := range strings.Fields(words) {
		out[strings.ToLower(word)] = struct{}{}
	}
	return out
}

// IsReservedSourceIdentifier reports whether name is reserved in an Apex
// declaration context. Salesforce permits most reserved words as method names.
func IsReservedSourceIdentifier(name string, methodName bool) bool {
	reserved := salesforceReservedIdentifiers
	if methodName {
		reserved = salesforceAlwaysKeywords
	}
	_, ok := reserved[strings.ToLower(name)]
	return ok
}
