package dml

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The builtin Account, Contact and Lead matching rules, as recovered from
// native Datacloud.FindDuplicates verdicts (docs/DUPLICATE_FUZZY_MATCHING.md,
// "Standard rules"). The items are the fuzzy methods of the custom rules, so a
// pair matches only when the item scorers of this package match it; the
// standard Contact name paths also recognize corroborated initials and
// surname edits in the Apex save and search rows.
//
// Every item compares two values with one of five relations: both present and
// matching, both present and different, the candidate blank, the existing
// record blank, both blank. Only a matching pair satisfies an item. Blank
// values never match, not even two blank ones.

type standardRelation int

const (
	standardNoMatch standardRelation = iota
	standardMatch
	standardCandidateBlank
	standardExistingBlank
	standardBothBlank
)

type standardItem struct {
	relation   standardRelation
	confidence float64
}

func (i standardItem) ok() bool { return i.relation == standardMatch }

func standardCompare(candidate, existing string, score func(a, b string) (bool, float64)) standardItem {
	candidate, existing = strings.TrimSpace(candidate), strings.TrimSpace(existing)
	switch {
	case candidate == "" && existing == "":
		return standardItem{relation: standardBothBlank}
	case candidate == "":
		return standardItem{relation: standardCandidateBlank}
	case existing == "":
		return standardItem{relation: standardExistingBlank}
	}
	candidate, existing = capFuzzyInput(candidate), capFuzzyInput(existing)
	if matched, confidence := score(candidate, existing); matched {
		return standardItem{relation: standardMatch, confidence: confidence}
	}
	return standardItem{relation: standardNoMatch}
}

func standardMethod(method string) func(a, b string) (bool, float64) {
	return func(a, b string) (bool, float64) {
		matched, confidence := fuzzyScorers[method](a, b, false)
		return matched, confidence
	}
}

func standardLastName(a, b string) (bool, float64) {
	if !standardLastNameGate(a, b) {
		return false, 0
	}
	return fuzzyLastName(a, b, false)
}

// standardExact compares two values without letter case or edge spaces.
func standardExact(a, b string) (bool, float64) {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)), 100
}

var (
	standardURLScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://`)
)

// standardZip: a five digit code with an optional hyphenated four digit
// extension. The first five digits weigh 90 and the extension 10: two
// extensions that differ score 95, an extension against none scores 100, and
// "94105 1234" with a space is not an extension at all. Any other value
// matches only itself.
func standardZip(a, b string) (bool, float64) {
	headA, extA, okA := standardZipParts(a)
	headB, extB, okB := standardZipParts(b)
	if !okA || !okB {
		return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " ")), 100
	}
	if headA != headB {
		return false, 0
	}
	if extA != "" && extB != "" && extA != extB {
		return true, 95
	}
	return true, 100
}

// standardZipParts splits "12345" or "12345-6789" into the five digits and the
// extension.
func standardZipParts(s string) (head, ext string, ok bool) {
	digits := func(t string) bool {
		for i := 0; i < len(t); i++ {
			if t[i] < '0' || t[i] > '9' {
				return false
			}
		}
		return true
	}
	switch {
	case len(s) == 5 && digits(s):
		return s, "", true
	case len(s) == 10 && s[5] == '-' && digits(s[:5]) && digits(s[6:]):
		return s[:5], s[6:], true
	}
	return "", "", false
}

// standardCompany is the Company Name method with a cheap necessary condition.
// Two names match only through their match key (the first five letters of the
// first two words), an acronym entry, or a hyphen spelling, so two names of
// seven characters or more that start with different letters match nothing
// unless one starts with a word the method drops.
func standardCompany(a, b string) (bool, float64) {
	if !standardCompanyGate(a, b) {
		return false, 0
	}
	return fuzzyCompany(a, b, false)
}

// standardCity is the City method with a cheap necessary condition: two
// values whose match keys agree start with the same letter or digit.
func standardCity(a, b string) (bool, float64) {
	if strings.EqualFold(a, b) {
		return true, 100
	}
	if !standardCityGate(a, b) {
		return false, 0
	}
	return fuzzyCity(a, b, false)
}

func firstASCIIAlnum(s string) byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 0x80:
			return 0
		case c >= 'A' && c <= 'Z':
			return c + 'a' - 'A'
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			return c
		}
	}
	return 0
}

// standardWebsite compares web addresses without letter case and a leading
// "www." ("www.acme" never finds "acme"). An address with a scheme also loses the scheme and a lone trailing
// slash; without a scheme the slash stays ("acme.com/" never matches
// "acme.com", "https://www.acme.com/" does).
func standardWebsite(a, b string) (bool, float64) {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if loc := standardURLScheme.FindStringIndex(s); loc != nil {
			s = strings.TrimSuffix(s[loc[1]:], "/")
		}
		if rest := strings.TrimPrefix(s, "www."); strings.Contains(rest, ".") {
			s = rest
		}
		return s
	}
	x, y := norm(a), norm(b)
	if x == y {
		return true, 100
	}
	// Two addresses that differ only by hyphens match at the edit similarity
	// of the pair, averaged with 100 ("acme-inc.com" and "acmeinc.com" score 96).
	if strings.Contains(x, "-") != strings.Contains(y, "-") && strings.ReplaceAll(x, "-", "") == strings.ReplaceAll(y, "-", "") {
		return true, averageWith100(editSimilarity(x, y))
	}
	return false, 0
}

// standardPhoneLineMismatch is the confidence of two numbers that agree up to
// the last four digits, by the object whose rule scores them.
var standardPhoneLineMismatch = map[string]float64{"account": 94, "contact": 97, "lead": 98}

func standardPhone(object string) func(a, b string) (bool, float64) {
	return func(a, b string) (bool, float64) {

		// Digits only: natively "503-555-O142" (a letter for a zero) finds
		// nothing, and a seven digit number matches the full number in the fit
		// rows but not in the holdout rows, so Glade leaves it unmatched.
		for _, s := range []string{a, b} {
			for _, r := range s {
				if !(r >= '0' && r <= '9') && !strings.ContainsRune("()+-./ \t", r) {
					return false, 0
				}
			}
		}
		if da, db := standardDigits(a), standardDigits(b); len(da) == 7 && da == db {
			// NATIVE two seven digit numbers with the same digits match
			// ("1234567" finds "123-4567"). Equal digits of another length do
			// not: "415-555-1234 x5" never finds "415-555-12345" and
			// "+44 20 7946 0958" never finds "442079460958".
			return true, 100
		}
		ca, aa, ea, la, okA := phoneParts(a)
		cb, ab, eb, lb, okB := phoneParts(b)
		if !okA || !okB {
			return false, 0
		}
		if ca != cb && ((ca != "" && ca != "1") || (cb != "" && cb != "1")) {
			return false, 0
		}
		if aa != ab || ea != eb {
			return false, 0
		}
		if la == lb {
			return true, 100
		}
		return true, standardPhoneLineMismatch[object]
	}
}

// standardStreet matches the streets the custom Street method matches and
// scores them by the sections that differ: a different suffix word costs 9
// points, a street name that is not the same costs a fifth of its dissimilarity,
// a different suite or unit costs 3. Natively "Main St" and "Main Street" are
// the same section values and score 100.
func standardStreet(a, b string) (bool, float64) {
	if ok, _ := fuzzyStreet(a, b, false); !ok {
		return false, 0
	}
	if strings.Contains(a, "#") || strings.Contains(b, "#") {
		// NATIVE: a street with a number sign finds only its own spelling
		// ("210 Elm St #4" finds neither "210 Elm St." nor "210 Elm Street Apt 4"),
		// although "Apt 3" finds "Apt 4" and "Ste 5" finds "Suite 6".
		if streetClean(a) != streetClean(b) {
			return false, 0
		}
	}
	pa, pb := parseStreet(a), parseStreet(b)
	confidence := 100.0
	if pa.suffix != "" && pb.suffix != "" && pa.suffix != pb.suffix {
		confidence *= 0.91
	}
	if pa.name != pb.name {
		confidence *= 1 - 0.2*(1-editSimilarity(pa.name, pb.name)/100)
	}
	if pa.suite != "" && pb.suite != "" && pa.suite != pb.suite {
		confidence *= 0.97
	}
	return true, confidence
}

func standardDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// product multiplies item confidences, each a percentage.
func standardProduct(confidences ...float64) float64 {
	out := 100.0
	for _, c := range confidences {
		out = out * c / 100
	}
	return out
}

func bestOf(items ...standardItem) (float64, bool) {
	best, found := 0.0, false
	for _, item := range items {
		if item.ok() && (!found || item.confidence > best) {
			best, found = item.confidence, true
		}
	}
	return best, found
}

// standardAccountMatch is the standard Account rule: the name and a city or
// ZIP code, or the website, a city or ZIP code, and a street or phone number.
// The Name path scores the product of the name and the better of city and ZIP;
// the Website path scores the product of the website and the better of street
// and phone, the city or ZIP code only gating it. The best path decides.
func standardAccountMatch(candidate standardAccountCandidate, existing storage.Record) (bool, float64) {
	// Fields of the candidate that are blank are never read from the existing
	// record: a blank candidate value satisfies no item.
	field := func(name string) string {
		v, ok := existing.Fields[name]
		if !ok {
			if v, ok = existing.GetField(name); !ok {
				return ""
			}
		}
		return workflowValueString(v)
	}
	var city, zip standardItem
	if candidate.city != "" {
		// The match keys decide before any scoring.
		if text := field("BillingCity"); text != "" && (strings.EqualFold(candidate.city, strings.TrimSpace(text)) || rawKeysOf(text).city == candidate.cityKeys.city) {
			city = standardCompare(candidate.city, text, standardCity)
		}
	}
	if candidate.zip != "" {
		zip = standardCompare(candidate.zip, field("BillingPostalCode"), standardZip)
	}
	cityOrZip, haveCZ := bestOf(city, zip)
	if !haveCZ {
		return false, 0
	}
	best, found := 0.0, false
	if candidate.name != "" {
		if text := field("Name"); text != "" && companyKeysMayMatch(candidate.nameKeys.company, rawKeysOf(text).company, candidate.name, text) {
			if name := standardCompare(candidate.name, text, standardCompany); name.ok() {
				best, found = standardProduct(name.confidence, cityOrZip), true
			}
		}
	}
	if candidate.website != "" {
		if website := standardCompare(candidate.website, field("Website"), standardWebsite); website.ok() {
			var street, phone standardItem
			if candidate.street != "" {
				street = standardCompare(candidate.street, field("BillingStreet"), standardStreet)
			}
			if candidate.phone != "" {
				phone = standardCompare(candidate.phone, field("Phone"), standardPhone("account"))
			}
			if streetOrPhone, ok := bestOf(street, phone); ok {
				if c := standardProduct(website.confidence, streetOrPhone); !found || c > best {
					best, found = c, true
				}
			}
		}
	}
	return found, best
}

// standardAccountCandidate is the candidate's trimmed fields, read once per scan.
type standardAccountCandidate struct {
	name, website, city, zip, street, phone string
	nameKeys, cityKeys                      standardRawKeys
}

func newStandardAccountCandidate(r storage.Record) (standardAccountCandidate, bool) {
	text := func(name string) string {
		text, _ := duplicateFieldText(r, name)
		return text
	}
	c := standardAccountCandidate{name: text("Name"), website: text("Website"), city: text("BillingCity"), zip: text("BillingPostalCode"), street: text("BillingStreet"), phone: text("Phone")}
	if c.name != "" {
		c.nameKeys = rawKeysOf(c.name)
	}
	if c.city != "" {
		c.cityKeys = rawKeysOf(c.city)
	}
	// Every path needs a city or ZIP code and a name or website: a candidate
	// without them matches nothing and scans nothing.
	return c, (c.city != "" || c.zip != "") && (c.name != "" || c.website != "")
}

// standardPerson is the part of a Contact or Lead the standard rules read.
type standardPerson struct {
	kind                             string // "contact" or "lead"
	first, last, email, phone, title string
	street, city, zip                string
	company, lastKey                 string
	accountID                        storage.ID
}

func standardPersonOf(org *storage.OrgState, object string, record storage.Record) standardPerson {
	field := func(names ...string) string {
		for _, name := range names {
			if v, ok := record.GetField(name); ok {
				if s := strings.TrimSpace(workflowValueString(v)); s != "" {
					return s
				}
			}
		}
		return ""
	}
	p := standardPerson{kind: strings.ToLower(object), first: field("FirstName"), last: field("LastName"), email: field("Email"), phone: field("Phone"), title: field("Title")}
	if p.kind == "lead" {
		p.street, p.city, p.zip, p.company = field("Street"), field("City"), field("PostalCode"), field("Company")
		return p
	}
	p.street, p.city, p.zip = field("MailingStreet"), field("MailingCity"), field("MailingPostalCode")
	if v, ok := record.GetField("AccountId"); ok && !duplicateBlank(v, ok) {
		p.accountID = storage.ID(workflowValueString(v))
		if org != nil {
			if account, ok := org.Objects["Account"].Records[p.accountID]; ok {
				if name, ok := account.GetField("Name"); ok {
					p.company = strings.TrimSpace(workflowValueString(name))
				}
			}
		}
	}
	return p
}

// standardPersonMatch is the standard Contact and Lead rule (the two share
// one structure, and a cross-object rule applies it to a record of the other
// object): the last name and first name, then an email address, a phone
// number, or the company with a title or a street and a city or ZIP code. A
// candidate without a first name still matches on the email address, as does
// a pair without first names; an existing record without one never matches a
// candidate that has one. The phone and company paths need both first names.
func standardPersonMatch(object string, candidate, existing standardPerson) (bool, float64) {
	last := standardCompare(candidate.last, existing.last, standardLastName)
	email := standardCompare(candidate.email, existing.email, standardExact)
	if candidate.kind == existing.kind && email.ok() && candidate.first != "" && strings.EqualFold(candidate.first, existing.first) {
		if !last.ok() {
			return standardCorroboratedSurname(candidate.last, existing.last)
		}
		if last.confidence < 100 {
			if standardIsTerminalSwap(candidate.last, existing.last) {
				return standardTerminalSwap(strings.ToLower(candidate.last), last.confidence)
			}
			// Some four-to-five trailing letters qualify via the inherited
			// phonetic key and keyboard item before the supplementary scorer.
			a, b := candidate.last, existing.last
			if len(a) > len(b) {
				a, b = b, a
			}
			if len(a) == 4 && len(b) == 5 && standardNameWord(a) && standardNameWord(b) && strings.EqualFold(a, b[:4]) {
				return true, standardTrailingLetterConfidence(strings.ToLower(a), last.confidence)
			}
		}
	}
	if !last.ok() {
		return false, 0
	}
	first := standardCompare(candidate.first, existing.first, standardMethod("firstname"))
	// The standard rule accepts one initial with an exact surname and email
	// or phone. A custom FirstName item does not match the same pairs.
	if !first.ok() && candidate.kind == existing.kind && strings.EqualFold(candidate.last, existing.last) && standardFirstInitial(candidate.first, existing.first) {
		if email.ok() {
			return true, 100
		}
		if candidate.phone != "" && candidate.phone == existing.phone {
			return standardPhone(object)(candidate.phone, existing.phone)
		}
	}
	best, found := 0.0, false
	take := func(c float64) {
		if !found || c > best {
			best, found = c, true
		}
	}
	switch first.relation {
	case standardMatch:
		firstConfidence := first.confidence
		if email.ok() {
			take(standardProduct(last.confidence, firstConfidence, email.confidence))
		}
		if phone := standardCompare(candidate.phone, existing.phone, standardPhone(object)); phone.ok() {
			take(standardProduct(last.confidence, firstConfidence, phone.confidence))
		}
		if confidence, ok := standardCompanyPath(candidate, existing); ok {
			take(standardProduct(last.confidence, firstConfidence, confidence))
		}
	case standardCandidateBlank, standardBothBlank:
		if email.ok() {
			take(standardProduct(last.confidence, email.confidence))
		}
	}
	return found, best
}

// standardEmailDomain is the part of an address after the last "@", in lower
// case; empty for a value without one.
func standardEmailDomain(email string) string {
	if i := strings.LastIndexByte(email, '@'); i >= 0 {
		return strings.ToLower(strings.TrimSpace(email[i+1:]))
	}
	return ""
}

// standardCompanyPath is the company branch: the company or the email domain
// is the same (NATIVE: two addresses at one domain stand in for the company),
// and the title is the same or the street and a city or ZIP code are. A lead
// candidate's company text never equals a contact's account.
func standardCompanyPath(candidate, existing standardPerson) (float64, bool) {
	confidence, sameCompany := 100.0, false
	switch {
	case candidate.kind == "contact" && existing.kind == "contact":
		sameCompany = candidate.accountID != "" && storage.IDsEqual(candidate.accountID, existing.accountID)
	case candidate.kind == "lead" && existing.kind == "contact":
	default:
		company := standardCompare(candidate.company, existing.company, standardCompany)
		sameCompany, confidence = company.ok(), company.confidence
	}
	if !sameCompany {
		domain := standardEmailDomain(candidate.email)
		if domain == "" || domain != standardEmailDomain(existing.email) {
			return 0, false
		}
		confidence = 100
	}
	if standardCompare(candidate.title, existing.title, standardExact).ok() {
		return confidence, true
	}
	street := standardCompare(candidate.street, existing.street, standardStreet)
	if street.ok() && (standardCompare(candidate.city, existing.city, standardCity).ok() || standardCompare(candidate.zip, existing.zip, standardZip).ok()) {
		return confidence, true
	}
	return 0, false
}
