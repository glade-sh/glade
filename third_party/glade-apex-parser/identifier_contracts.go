package apexast

import (
	"fmt"
	"unicode"
)

// MaxApexSourceIdentifierLength is the Salesforce Apex identifier length limit.
const MaxApexSourceIdentifierLength = 255

// ValidateSourceIdentifier reports whether name is a legal Apex source
// identifier. Schema and API reference names are not validated here; callers
// must only apply this to Apex declaration/rename targets.
func ValidateSourceIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
	}
	if len(name) > MaxApexSourceIdentifierLength {
		return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
	}
	runes := []rune(name)
	if !unicode.IsLetter(runes[0]) || runes[0] > unicode.MaxASCII {
		return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
	}
	for i, r := range runes {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
		}
		if r == '_' && i+1 < len(runes) && runes[i+1] == '_' {
			return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
		}
	}
	if runes[len(runes)-1] == '_' {
		return fmt.Errorf("%s", sourceIdentifierErrorMessage(name))
	}
	return nil
}

func sourceIdentifierErrorMessage(name string) string {
	return "Invalid identifier: " + name
}
