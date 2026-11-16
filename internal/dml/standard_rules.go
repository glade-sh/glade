package dml

import (
	"regexp"
	"strings"
	"unicode"

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

// standardFirstName is the First Name method as the standard rules apply it.
// It keeps the whole-name match key and Jaro-Winkler on the folded text: the
// standard rules' first-name key differs from a custom FirstName rule's
// (their own rows key on the first letter where an email agrees), and the
// first-word key and Initials rule of fuzzyFirstName are captured only for
// custom rules. A name and the same name followed by a second word or by
// punctuation share the key ("Billy Bob" finds "Billy", "Jane." finds "Jane").
func standardFirstName(a, b string) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if x == y || nameVariantOf(x, y) {
		return true, 100
	}
	if kx, ky := firstNameKey(x), firstNameKey(y); (kx == "" || kx != ky || nameKeptApart[orderedPair(x, y)]) && standardFirstNameHead(x) != y && standardFirstNameHead(y) != x {
		return false, 0
	}
	score := jaroWinkler(x, y)
	return score >= firstNameJaroWinkler-1e-9, averageWith100(100 * score)
}

// standardFirstNameHead is the first word of a folded name with leading and
// trailing punctuation trimmed.
func standardFirstNameHead(s string) string {
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	return strings.TrimFunc(s, unicode.IsPunct)
}

// standardLastName is the Last Name method as the standard rules apply it:
// the key is the whole name (standardLastNameGate), and two Metaphone 3 codes
// that agree score 100 only when the names' spaces and hyphens agree, so
// "Smith Jones" does not find "Smith-Jones". The custom Last Name method keys
// the last word instead (fuzzyLastName); the standard rows ("43" finds "42",
// the Shapes.*.Last rows) are fitted to this whole-name form.
func standardLastName(a, b string) (bool, float64) {
	if !standardLastNameGate(a, b) {
		return false, 0
	}
	x, y := foldText(a), foldText(b)
	if x == y {
		return true, 100
	}
	x, y = strings.ReplaceAll(x, "'", ""), strings.ReplaceAll(y, "'", "")
	if x == y || standardSurnameSeparators(x) == standardSurnameSeparators(y) && lastNameMetaphone3(x) == lastNameMetaphone3(y) {
		return true, 100
	}
	return lastNameKeyboard(x, y)
}

func standardSurnameSeparators(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r == ' ' || r == '-' {
			out.WriteRune(r)
		}
	}
	return out.String()
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
				street = standardCompare(candidate.street, field("BillingStreet"), standardAccountStreet)
			}
			if candidate.phone != "" {
				phone = standardCompare(candidate.phone, field("Phone"), standardAccountPhone)
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
// object). The rule is an OR of AND groups, each found through its own
// predefined match key (Salesforce Help, "Standard Contact Matching Rule and
// Standard Lead Matching Rule"):
//
//   - the email path, keyed on Email alone: the first and last names are
//     scored without a key (standardEmailPath);
//   - the phone path, keyed on First_Name (1,1) Last_Name Phone;
//   - the company path, with a title or a street and a city or ZIP code
//     (its key, First_Name (1,1) Last_Name Company (2,5), is not modeled: no
//     row measures it, so it keeps the standard First Name item).
//
// The phone and company paths need the surname key (standardLastName) and
// both first names. A cross-object rule keeps the surname key and the
// standard First Name item on every path.
// The best path decides.
func standardPersonMatch(object string, candidate, existing standardPerson) (bool, float64) {
	best, found := 0.0, false
	take := func(c float64) {
		if !found || c > best {
			best, found = c, true
		}
	}
	email := standardCompare(candidate.email, existing.email, standardExact)
	if email.ok() && candidate.kind == existing.kind {
		if matched, confidence, decided := standardEmailPath(candidate, existing); decided {
			if !matched {
				return false, 0
			}
			take(confidence)
		}
	}
	last := standardCompare(candidate.last, existing.last, standardLastName)
	if !last.ok() {
		return found, best
	}
	if email.ok() && candidate.kind != existing.kind {
		// A cross-object pair keeps the keyed email path.
		first := standardCompare(candidate.first, existing.first, standardFirstName)
		switch first.relation {
		case standardMatch:
			take(standardProduct(last.confidence, first.confidence, email.confidence))
		case standardCandidateBlank, standardBothBlank:
			take(standardProduct(last.confidence, email.confidence))
		}
	}
	if phone := standardCompare(candidate.phone, existing.phone, standardPhone(object)); phone.ok() {
		if candidate.kind != existing.kind {
			// No row measures a cross-object phone path: it keeps the
			// standard First Name item and its confidence.
			if first := standardCompare(candidate.first, existing.first, standardFirstName); first.ok() {
				take(standardProduct(last.confidence, first.confidence, phone.confidence))
			}
		} else if first, ok := standardInitialKeyedFirst(candidate.first, existing.first); ok {
			take(standardProduct(last.confidence, first, phone.confidence))
		}
	}
	// No row measures the company path's first names: it keeps the
	// standard First Name item and its confidence.
	if first := standardCompare(candidate.first, existing.first, standardFirstName); first.ok() {
		if confidence, ok := standardCompanyPath(candidate, existing); ok {
			take(standardProduct(last.confidence, first.confidence, confidence))
		}
	}
	return found, best
}

// standardEmailPath is the email path of the standard Contact and Lead rule.
// Its match key is the email address alone, so neither name is keyed: the
// last name is scored by the Last Name algorithms (B2 s_suf: "McAllen" finds
// "Mc Allen", "van der Berg" finds "Vanderberg", "Harlow" finds "Harlow III";
// the standard fixture's Field.Contact.Last: "Kovalski" finds "Kowalski") and
// the first name by standardFirstNameScore. Blank names are ignored when the
// grouping includes Email: two blank last names agree, and a blank candidate
// first name matches any existing one. A surname pair inside the measured
// exact-first-name panels keeps the panels' verdict whatever the first names.
// decided is false when the email path leaves the pair to the other paths (an
// existing record without a first name, first names that do not match).
func standardEmailPath(candidate, existing standardPerson) (matched bool, confidence float64, decided bool) {
	lastConfidence := 100.0
	switch {
	case candidate.last == "" && existing.last == "":
		// Two blank last names are ignored only beside two first names
		// (Field.Contact.Last and Field.Lead.Last: "Jonathan" with no last
		// name finds itself at 100); no row shows four blank names.
		if candidate.first == "" || existing.first == "" {
			return false, 0, true
		}
	case candidate.last == "" || existing.last == "":
		return false, 0, true
	default:
		// The exact-first-name surname panels (duplicate_save_behavior,
		// surname_keyboard, surname_substitution) measure these keyboard
		// geometries. Their verdict on the surname stands whatever the first
		// names: a pair they keep apart with equal first names is not found
		// through a blank or a merely similar first name either.
		panel := false
		if matched, confidence := standardCorroboratedSurname(candidate.last, existing.last); matched {
			lastConfidence, panel = confidence, true
		} else if last := standardCompare(candidate.last, existing.last, standardLastName); last.ok() {
			if last.confidence < 100 && standardIsTerminalSwap(candidate.last, existing.last) {
				matched, confidence := standardTerminalSwap(strings.ToLower(candidate.last), last.confidence, true)
				if !matched {
					return false, 0, true
				}
				lastConfidence, panel = confidence, true
			}
		} else if standardSurnamePanelShape(candidate.last, existing.last) {
			return false, 0, true
		}
		if !panel {
			var matched bool
			if standardHasLetter(candidate.last) && standardHasLetter(existing.last) {
				matched, lastConfidence = lastNameAlgorithms(foldText(capFuzzyInput(candidate.last)), foldText(capFuzzyInput(existing.last)))
			} else {
				// A name without a letter has no phonetic code: the keyed item
				// decides ("42" finds "43" at 99 and never "王"; Shapes.Contact.Last).
				last := standardCompare(candidate.last, existing.last, standardLastName)
				matched, lastConfidence = last.ok(), last.confidence
			}
			if !matched {
				return false, 0, true
			}
		}
	}
	switch {
	case candidate.first == "":
		return true, lastConfidence, true
	case existing.first == "":
		return false, 0, false
	}
	matched, score := standardFirstNameScore(candidate.first, existing.first)
	if !matched {
		return false, 0, false
	}
	return true, standardProduct(lastConfidence, standardFirstNameConfidence(score)), true
}

// standardHasLetter reports a value with at least one letter.
func standardHasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

// standardFirstNameConfidence is the share of a standard first-name score in
// the rule confidence: the item is one of three that the rule averages, so
// a score s counts (200 + s) / 3 (B2 s_in_c: "Y R" and "Y R S" score 97 on
// the standard rule against 95 on a custom one; the fixture's
// NameConfidence: "Jon" and "Jo" with an exact surname score 97).
func standardFirstNameConfidence(score float64) float64 {
	return (200 + score) / 3
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

// The standard Account rule accepts one house-number digit edit, with the
// street name, suffix and unit unchanged, at 97 (street/phone grid S1/S3).
// Other street changes keep the shared street scorer's verdict.
func standardAccountStreet(a, b string) (bool, float64) {
	if matched, confidence := standardStreet(a, b); matched {
		return true, confidence
	}
	x, y := parseStreet(a), parseStreet(b)
	// The captured number edits preserve the rest of the spelling. Earlier
	// street panels reject number edits paired with a suffix spelling change.
	aw, bw := strings.Fields(a), strings.Fields(b)
	if len(aw) < 2 || len(bw) < 2 || !strings.EqualFold(strings.Join(aw[1:], " "), strings.Join(bw[1:], " ")) {
		return false, 0
	}
	if x.number == "" || y.number == "" || x.name == "" || x.name != y.name || x.suffix != y.suffix || x.suite != "" || y.suite != "" || strings.ContainsAny(a+b, "#") || standardDigits(x.number) != x.number || standardDigits(y.number) != y.number {
		return false, 0
	}
	if len(x.number) > len(y.number) {
		x, y = y, x
	}
	if len(x.number) == len(y.number) {
		if x.number[:len(x.number)-1] == y.number[:len(y.number)-1] && x.number != y.number {
			return true, averageWith100(editSimilarity(streetClean(a), streetClean(b)))
		}
	} else if len(y.number) == len(x.number)+1 && strings.HasPrefix(y.number, x.number) {
		return true, averageWith100(editSimilarity(streetClean(a), streetClean(b)))
	}
	return false, 0
}

// Seven-digit local numbers match the same exchange and line on an Account.
// Extensions and other invalid characters retain the shared phone rejection.
func standardAccountPhone(a, b string) (bool, float64) {
	valid := func(s string) bool {
		for _, r := range s {
			if !(r >= '0' && r <= '9') && !strings.ContainsRune("()+-./ \t", r) {
				return false
			}
		}
		return true
	}
	if valid(a) && valid(b) {
		x, y := standardDigits(a), standardDigits(b)
		if len(x) > len(y) {
			x, y = y, x
		}
		if len(x) == 7 && (len(y) == 10 || len(y) == 11 && y[0] == '1') && strings.HasSuffix(y, x) {
			return true, 100
		}
	}
	return standardPhone("account")(a, b)
}
