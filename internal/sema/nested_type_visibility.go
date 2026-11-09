package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// nestedTypeVisibilityCode carries "Type is not visible" for a nested source
// type spelled from outside its top-level type.
const nestedTypeVisibilityCode = "GLADESEMA036"

// nestedTypeVisibilityMessage reports the native rejection for a type text
// that names a private or default-access nested source type from code outside
// its top-level type. Native rows: testdata/conformance/nested_type_visibility
// (owner and plain orgs, API 62/67, anonymous and named): a nested class,
// enum or interface without public or global is visible inside its top-level
// type only; @TestVisible lifts that for @IsTest classes. The message spells
// the type in its declared case, prefixed with the project namespace, and
// keeps a generic wrapper around it.
func (a *Analyzer) nestedTypeVisibilityMessage(model *semaTypeMemberView, owner typesys.TypeSymbol, typeText string) (string, bool) {
	message, _, hidden := a.nestedTypeVisibility(model, owner, typeText)
	return message, hidden
}

// nestedTypeConstructionMessage names only the hidden type for a `new`
// expression: native spells `new List<Outer.Hidden>()` as the bare type.
func (a *Analyzer) nestedTypeConstructionMessage(model *semaTypeMemberView, owner typesys.TypeSymbol, typeText string) (string, bool) {
	_, displays, hidden := a.nestedTypeVisibility(model, owner, typeText)
	if !hidden {
		return "", false
	}
	return "Type is not visible: " + displays[0], true
}

func (a *Analyzer) nestedTypeVisibility(model *semaTypeMemberView, owner typesys.TypeSymbol, typeText string) (string, []string, bool) {
	if a == nil || model == nil {
		return "", nil, false
	}
	if !strings.Contains(typeText, ".") && !a.ownerInheritsSource(model, owner) {
		return "", nil, false
	}
	message := strings.Join(strings.Fields(typeText), "")
	var displays []string
	for _, ref := range extractTypeNames(typeText) {
		display, bad := a.hiddenNestedTypeDisplay(model, owner, ref)
		if !bad {
			continue
		}
		displays = append(displays, display)
		message = replaceTypeToken(message, strings.Join(strings.Fields(ref), ""), display)
	}
	if len(displays) == 0 {
		return "", nil, false
	}
	return "Type is not visible: " + message, displays, true
}

func (a *Analyzer) hiddenNestedTypeDisplay(model *semaTypeMemberView, owner typesys.TypeSymbol, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if root, rest, ok := strings.Cut(ref, "."); ok && a.namespace != "" && strings.EqualFold(root, a.namespace) {
		ref = rest
	}
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		return a.inheritedHiddenNestedDisplay(model, owner, ref)
	}
	top, ok := model.lookupName(parts[0])
	if !ok || top.platform || top.dependency || top.sobject || strings.Contains(top.name, ".") {
		return "", false
	}
	ownerTop := owner.Name
	if dot := strings.IndexByte(ownerTop, '.'); dot >= 0 {
		ownerTop = ownerTop[:dot]
	}
	sameTree := strings.EqualFold(ownerTop, top.name)
	ownerIsTest := false
	if !sameTree {
		if ownerMembers, found := model.lookupName(ownerTop); found {
			ownerIsTest = ownerMembers.isTest
		}
		ownerIsTest = ownerIsTest || owner.IsTest
	}
	display := top.name
	for end := 2; end <= len(parts); end++ {
		members, found := model.lookupName(strings.Join(parts[:end], "."))
		if !found || members.platform || members.dependency || strings.Count(members.name, ".") != end-1 {
			return "", false
		}
		display = members.name
		if hasModifier(members.modifiers, "public") || hasModifier(members.modifiers, "global") {
			continue
		}
		if sameTree || members.testVisible && ownerIsTest {
			continue
		}
		if a.namespace != "" {
			display = a.namespace + "." + display
		}
		return display, true
	}
	return "", false
}

func nestedTypeVisibilityDiagnostic(typ typesys.TypeSymbol, message string, rng *diagnostic.Range) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: nestedTypeVisibilityCode, Message: message, NativeMessage: message, File: typ.File, Range: rng}
}

// dropShadowedNestedVisibility removes the bare hidden-type diagnostic that a
// generic declaration on the same line already names: native reports
// `List<Outer.Hidden> l = new List<Outer.Hidden>()` once.
func dropShadowedNestedVisibility(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	wrapped := map[string]bool{}
	for _, d := range diagnostics {
		if d.Code != nestedTypeVisibilityCode || d.Range == nil {
			continue
		}
		text := strings.TrimPrefix(semaNativeDiagnosticText(d), "Type is not visible: ")
		if open := strings.IndexByte(text, '<'); open >= 0 && strings.HasSuffix(text, ">") {
			for _, inner := range strings.FieldsFunc(text[open+1:len(text)-1], func(r rune) bool { return r == ',' || r == '<' || r == '>' }) {
				wrapped[fmt.Sprintf("%s|%d|%s", d.File, d.Range.Start.Line, inner)] = true
			}
		}
	}
	if len(wrapped) == 0 {
		return diagnostics
	}
	out := diagnostics[:0:0]
	for _, d := range diagnostics {
		if d.Code == nestedTypeVisibilityCode && d.Range != nil {
			text := strings.TrimPrefix(semaNativeDiagnosticText(d), "Type is not visible: ")
			if !strings.Contains(text, "<") && wrapped[fmt.Sprintf("%s|%d|%s", d.File, d.Range.Start.Line, text)] {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

// replaceTypeToken replaces the first whole-identifier occurrence of old.
func replaceTypeToken(text, old, replacement string) string {
	isIdent := func(b byte) bool {
		return b == '_' || b == '.' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	}
	for from := 0; from <= len(text)-len(old); {
		at := strings.Index(text[from:], old)
		if at < 0 {
			break
		}
		at += from
		end := at + len(old)
		if (at == 0 || !isIdent(text[at-1])) && (end == len(text) || !isIdent(text[end])) {
			return text[:at] + replacement + text[end:]
		}
		from = at + 1
	}
	return text
}

// inheritedHiddenNestedDisplay reports a bare name that a class inherits from
// its superclass as a nested type it may not see. Native rows
// (standard_object_shape_compile I01-I16): a subclass of Outer cannot use a
// private or default-access nested type of Outer, spelled bare or qualified;
// the message names the qualified type.
func (a *Analyzer) inheritedHiddenNestedDisplay(model *semaTypeMemberView, owner typesys.TypeSymbol, ref string) (string, bool) {
	members, found := a.scopedNestedType(model, owner.Name, ref)
	if !found || members.platform || members.dependency || members.sobject {
		return "", false
	}
	if hasModifier(members.modifiers, "public") || hasModifier(members.modifiers, "global") {
		return "", false
	}
	ownerTop := owner.Name
	if dot := strings.IndexByte(ownerTop, '.'); dot >= 0 {
		ownerTop = ownerTop[:dot]
	}
	top := members.name
	if dot := strings.IndexByte(top, '.'); dot >= 0 {
		top = top[:dot]
	}
	if strings.EqualFold(ownerTop, top) {
		return "", false
	}
	ownerIsTest := owner.IsTest
	if ownerMembers, known := model.lookupName(ownerTop); known {
		ownerIsTest = ownerIsTest || ownerMembers.isTest
	}
	if members.testVisible && ownerIsTest {
		return "", false
	}
	display := members.name
	if a.namespace != "" {
		display = a.namespace + "." + display
	}
	return display, true
}
