package storage

import "strings"

// NamedApexReferenceObjectNames returns catalog reference targets whose source
// types are available in named Apex bodies but rejected by anonymous Apex.
// They remain outside KnownStandardObjectNames. Native observations live in
// apextest/testdata/conformance/reference_only_names at API 62 and 67.
func NamedApexReferenceObjectNames() []string {
	return []string{"FieldServiceMobileExtension"}
}

// ResolveNamedApexReferenceObjectName resolves an unqualified name without
// changing global standard-object registration or source-context visibility.
func ResolveNamedApexReferenceObjectName(name string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(name), "FieldServiceMobileExtension") {
		return "FieldServiceMobileExtension", true
	}
	return "", false
}
