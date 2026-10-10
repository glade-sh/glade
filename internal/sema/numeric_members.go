package sema

import (
	"github.com/glade-sh/glade/internal/ir"
	"strings"
)

// Reject calls outside the public Decimal/Double surface even when the
// permissive platform catalog does not contain a resolved member.
func semaNumericCallRejected(receiverType, method string, args []ir.Expr) bool {
	typ := strings.ToLower(semaCanonicalPlatformAlias(receiverType))
	// Flattened Math constant calls retain Math as the receiver and put the
	// constant in the method path. PI and E have the Double member contract.
	if typ == "math" {
		constant, member, ok := strings.Cut(method, ".")
		if !ok || (!strings.EqualFold(constant, "PI") && !strings.EqualFold(constant, "E")) {
			return false
		}
		typ, method = "double", member
	}
	if typ != "decimal" && typ != "double" {
		return false
	}
	name := strings.ToLower(method)
	if name == "valueof" {
		return len(args) == 1 && args[0].Kind == ir.ExprLiteral && strings.EqualFold(args[0].Value, "null")
	}
	switch name {
	case "equals", "hashcode", "tostring", "format", "intvalue", "longvalue", "round":
		return false
	case "abs", "divide", "doublevalue", "pow", "precision", "scale", "setscale", "striptrailingzeros", "toplainstring":
		return typ == "double"
	default:
		return true
	}
}
