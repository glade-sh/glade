package sema

import (
	"fmt"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func isRemoteActionMethod(member typesys.MemberSymbol) bool {
	return member.Kind == apexast.DeclarationMethod && hasAnnotation(member.Annotations, "RemoteAction")
}

// The e_annotation_duplicate_method case also covers an unannotated duplicate of
// the annotated original. Ordinary duplicates retain their existing text.
func remoteActionDuplicateMethodDiagnostic(typ typesys.TypeSymbol, member, previous typesys.MemberSymbol) diagnostic.Diagnostic {
	d := duplicateMemberDiagnostic(typ, member, previous.Kind)
	if isRemoteActionMethod(member) || isRemoteActionMethod(previous) {
		d.NativeMessage = fmt.Sprintf("Method already defined: %s %s from the type %s", member.Name, nativeLifecycleSignature(typ.Name, member), typ.Name)
	}
	return d
}

// The e_annotation_protected case captures both class errors, with no additional
// RemoteAction visibility error. Only remoted declarations enter the new path.
func remoteActionProtectedMethodDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	d := declarationContractDiagnostic(typ, member.Range, fmt.Sprintf("protected method %q cannot be static", member.Name))
	if !isRemoteActionMethod(member) {
		return []diagnostic.Diagnostic{d}
	}
	d.NativeMessage = "protected methods cannot be static"
	if hasModifier(typ.Modifiers, "virtual") || hasModifier(typ.Modifiers, "abstract") || hasModifier(member.Modifiers, "override") {
		return []diagnostic.Diagnostic{d}
	}
	visibility := declarationContractDiagnostic(typ, member.Range, "New protected methods cannot be defined in non-virtual classes")
	visibility.NativeMessage = visibility.Message
	return []diagnostic.Diagnostic{visibility, d}
}

// The e_annotation_bad_type case supplies the native diagnostic for a remoted
// return type; fields and ordinary methods preserve their existing projection.
func remoteActionInvalidTypeMessage(member typesys.MemberSymbol, typeName string) string {
	if isRemoteActionMethod(member) {
		return "Invalid type: " + typeName
	}
	return ""
}
