package dml

import (
	"strings"
	"sync"
)

// The standard rules score a candidate against every existing record, so the
// per-value work (words of a company name, the city key, the phonetic key of a
// last name) is cached by the value's text. A cache keyed by an immutable
// string needs no invalidation; each cache is emptied when it passes
// standardKeyCacheLimit entries.
const standardKeyCacheLimit = 1 << 18

type standardKeyCache[V any] struct {
	mu sync.RWMutex
	m  map[string]V
}

func (c *standardKeyCache[V]) get(key string, compute func(string) V) V {
	c.mu.RLock()
	v, ok := c.m[key]
	c.mu.RUnlock()
	if ok {
		return v
	}
	v = compute(key)
	c.mu.Lock()
	if c.m == nil || len(c.m) >= standardKeyCacheLimit {
		c.m = make(map[string]V)
	}
	c.m[key] = v
	c.mu.Unlock()
	return v
}

type companyKeyInfo struct {
	key, joined string
}

var (
	companyKeyCache  standardKeyCache[companyKeyInfo]
	cityKeyCache     standardKeyCache[string]
	lastNameKeyCache standardKeyCache[string]
)

func companyKeyOf(s string) companyKeyInfo {
	return companyKeyCache.get(s, func(s string) companyKeyInfo {
		words := companyWords(s)
		return companyKeyInfo{key: companyKey(words), joined: strings.Join(words, " ")}
	})
}

// standardCompanyGate reports whether two company names can match at all:
// they match only through the same match key (equal text has equal words) or
// an acronym entry (fuzzyCompany).
func standardCompanyGate(a, b string) bool {
	x, y := companyKeyOf(a), companyKeyOf(b)
	if x.key == y.key {
		return true
	}
	return companyAcronyms[x.joined] == y.joined || companyAcronyms[y.joined] == x.joined
}

// companyKeysMayMatch is standardCompanyGate on cached keys; the texts are
// only needed for the equal-text case, which has equal keys.
func companyKeysMayMatch(x, y companyKeyInfo, _, _ string) bool {
	return x.key == y.key || companyAcronyms[x.joined] == y.joined || companyAcronyms[y.joined] == x.joined
}

// standardCityGate: two cities match only through the same fold or the same
// six-letter key.
func standardCityGate(a, b string) bool {
	x := cityKeyCache.get(a, func(s string) string { return cityKey(foldText(s)) })
	y := cityKeyCache.get(b, func(s string) string { return cityKey(foldText(s)) })
	return x == y
}

// standardLastNameGate: two last names match only through the same fold or
// the same double metaphone key.
func standardLastNameGate(a, b string) bool {
	x, y := lastNameCode(foldText(a)), lastNameCode(foldText(b))
	if x != y {
		return false
	}
	// Names with no code (Han, Cyrillic, digits only) all match each other,
	// but NATIVE a digits-only name ("42") never finds a name in letters
	// ("Иванов", "王").
	return x != "" || standardDigitsOnly(a) == standardDigitsOnly(b)
}

func standardDigitsOnly(s string) bool {
	seen := false
	for _, r := range s {
		if r == ' ' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
		seen = true
	}
	return seen
}

// Raw-text keys: one cache lookup per existing record, none of the folding
// that the lookup would repeat.
type standardRawKeys struct {
	company companyKeyInfo
	city    string
	last    string
}

var rawKeyCache standardKeyCache[standardRawKeys]

func rawKeysOf(s string) standardRawKeys {
	return rawKeyCache.get(s, func(s string) standardRawKeys {
		return standardRawKeys{company: companyKeyOf(s), city: cityKey(foldText(s)), last: lastNameCode(foldText(s))}
	})
}
