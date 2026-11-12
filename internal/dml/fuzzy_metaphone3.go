package dml

import (
	"strings"

	"github.com/dlclark/metaphone3"
)

// lastNameMetaphone3Equal reports two names whose Metaphone 3 primary codes
// are equal, which the platform scores 100 for the Last Name method. The
// encoder runs in exact mode (B, D and G stay apart from P, T and K: "Carter"
// does not find "Carder" at 100, "Reid" does not find "Right"), leaves
// non-initial vowels out of the code and keeps the library default length of
// eight characters; every option combination was measured against the native
// surname rows and this one matches the most of them with no false match.
//
// The code does not see a space or a hyphen, but natively "Smith Jones" does
// not find "Smith-Jones", "De La Cruz" does not find "Delacruz" and
// "Mc Donald" does not find "McDonald", so two names match only when their
// separators agree.
func lastNameMetaphone3Equal(x, y string) bool {
	if lastNameSeparators(x) != lastNameSeparators(y) {
		return false
	}
	return lastNameMetaphone3(x) == lastNameMetaphone3(y)
}

// lastNameMetaphone3 returns the primary Metaphone 3 code. An Encoder holds
// state and is not safe for concurrent use, so each call takes its own.
func lastNameMetaphone3(s string) string {
	encoder := &metaphone3.Encoder{EncodeExact: true}
	primary, _ := encoder.Encode(s)
	return primary
}

func lastNameSeparators(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r == ' ' || r == '-' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
