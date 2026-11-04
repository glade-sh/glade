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

// NamedApexReferenceObjectDescribe returns the captured describe identity of a
// named-source reference object: the label, plural label and key prefix.
// Native describes also report no create, update, delete or read access and an
// empty field map (reference_only_surfaces, Winter '27 org, API 62-67).
func NamedApexReferenceObjectDescribe(name string) (label, pluralLabel, keyPrefix string, ok bool) {
	canonical, ok := ResolveNamedApexReferenceObjectName(name)
	if !ok {
		return "", "", "", false
	}
	switch canonical {
	case "FieldServiceMobileExtension":
		return "Field Service Mobile Extension", "Field Service Mobile Extensions", "1ME", true
	}
	return "", "", "", false
}
