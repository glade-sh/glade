package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native Page tokens reject absent metadata during compilation.
// Use the index's captured aliases; Page-valued bindings and source types keep
// ordinary member lookup, including when they shadow the platform token root.
func (a *Analyzer) pageReferenceDiagnostic(typ typesys.TypeSymbol, path string, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	if !a.visualforcePagesKnown {
		return diagnostic.Diagnostic{}, false
	}
	root, name, ok := strings.Cut(path, ".")
	if !ok || !strings.EqualFold(root, "Page") || !simpleIdentifierPattern.MatchString(name) || strings.EqualFold(name, "class") {
		return diagnostic.Diagnostic{}, false
	}
	if _, bound := scope.lookup(root); bound {
		return diagnostic.Diagnostic{}, false
	}
	resolved := resolveNestedTypeName(model, typ.Name, root)
	if resolved == "" {
		resolved = root
	}
	if members, exists := model.lookup(normalizeName(resolved)); exists && !members.platform {
		return diagnostic.Diagnostic{}, false
	}
	if a.visualforcePageNames[normalizeName(name)] {
		return diagnostic.Diagnostic{}, false
	}
	item := diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA013",
		Message:  "Page does not exist: " + name,
		File:     typ.File,
		Range:    semaRange(source, start, start+len(path)),
	}
	if !scope.anonymous && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
		item.NativeMessage = "Page " + name + " does not exist"
	}
	return item, true
}
