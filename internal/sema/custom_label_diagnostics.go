package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaCustomLabelDiagnostic(typ typesys.TypeSymbol, name string, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	if model == nil || model.state == nil || model.state.base == nil || !model.state.base.customLabelsKnown || !semaIRCustomLabelReference(name, scope, model, typ.Name) {
		return diagnostic.Diagnostic{}, false
	}
	root, _, _ := strings.Cut(strings.TrimSpace(name), ".")
	if semaProjectTypeShadowsPlatform(model, root) {
		return diagnostic.Diagnostic{}, false
	}
	label := strings.TrimSpace(name[strings.LastIndexByte(name, '.')+1:])
	if model.state.base.customLabelNames[normalizeName(label)] {
		return diagnostic.Diagnostic{}, false
	}
	message := "External string does not exist: " + label
	end := start + max(1, len(name))
	if at, last, found := semaIRNameSpan(source, start, name); found {
		start, end = at, last
	}
	return diagnostic.Diagnostic{
		Severity:      diagnostic.Error,
		Code:          "GLADESEMA013",
		Message:       message,
		NativeMessage: message,
		File:          typ.File,
		Range:         semaRange(source, start, end),
	}, true
}
