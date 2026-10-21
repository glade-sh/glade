package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// C194S051/S052: deployed code rejects an Account when-binding immediately
// followed by an enclosing Account or Integer local of the same name.
// Anonymous code accepts both units. Keep visibility and other selector,
// branch, following-statement and local-type shapes unchanged.
func forLoopScopeSwitchDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, following []ir.Instruction, scope irSemaScope, selectorType string, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if scope.anonymous || !forLoopScopeCapturedAPI(typ.EffectiveAPIVersion) ||
		!strings.EqualFold(selectorType, "SObject") || len(inst.Cases) != 1 ||
		len(inst.Cases[0].Exprs) != 1 || len(following) == 0 {
		return nil
	}
	caseType, name, ok := irSwitchTypeCase(inst.Cases[0].Exprs[0])
	local := following[0]
	caseType = resolveNestedTypeReference(model, typ.Name, caseType)
	local.Type = resolveNestedTypeReference(model, typ.Name, local.Type)
	if !ok || !strings.EqualFold(caseType, "Account") || local.Op != ir.OpDeclare ||
		normalizeName(local.Name) != normalizeName(name) ||
		(!strings.EqualFold(local.Type, "Account") && !strings.EqualFold(local.Type, "Integer")) ||
		semaProjectTypeShadowsPlatform(model, selectorType) || semaProjectTypeShadowsPlatform(model, caseType) || semaProjectTypeShadowsPlatform(model, local.Type) {
		return nil
	}
	return []diagnostic.Diagnostic{irRedeclareDiagnostic(typ, member, local.Name, local.Pos, bodyOffset, source)}
}
