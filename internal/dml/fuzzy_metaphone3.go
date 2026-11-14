package dml

import "github.com/dlclark/metaphone3"

// lastNameMetaphone3 returns the primary Metaphone 3 code; two last names
// with equal codes score 100 for the Last Name method. The encoder runs in
// exact mode (B, D and G stay apart from P, T and K: "Carter" does not find
// "Carder" at 100, "Reid" does not find "Right"), leaves non-initial vowels
// out of the code and keeps the library default length of eight characters;
// every option combination was measured against the native surname rows and
// this one matches the most of them with no false match. The standard rules
// also require equal separators (standardLastName). An Encoder holds
// state and is not safe for concurrent use, so each call takes its own.
func lastNameMetaphone3(s string) string {
	return metaphone3CodeCache.get(s, func(s string) string {
		encoder := &metaphone3.Encoder{EncodeExact: true}
		primary, _ := encoder.Encode(s)
		return primary
	})
}

// metaphone3CodeCache and firstNameCodeCache memoize the phonetic codes, pure
// functions of the text, that every standard-rule scan asks for once per
// existing record (standard_keys.go).
var (
	metaphone3CodeCache standardKeyCache[string]
	firstNameCodeCache  standardKeyCache[string]
)
