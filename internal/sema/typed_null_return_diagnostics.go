package sema

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The captured row already rejects this return on both routes. Supply only its
// captured text; keep the editor explanation and the semantic rejection intact.
func capturedListReturnConversion(returnType, valueType string, model *semaTypeMemberView) string {
	toBase, toArgs := semaGenericBaseAndArgs(returnType)
	fromBase, fromArgs := semaGenericBaseAndArgs(valueType)
	if !strings.EqualFold(toBase, "List") || !strings.EqualFold(fromBase, "List") ||
		len(toArgs) != 1 || len(fromArgs) != 1 || !strings.EqualFold(fromArgs[0], "String") {
		return ""
	}
	target, _, known := semaLookupTypeMembers(model, toArgs[0])
	if !known || target.kind != apexast.DeclarationClass || target.platform || target.sobject {
		return ""
	}
	return "Illegal conversion from " + valueType + " to " + returnType
}

// The named the captured row test reports the invalid zero-argument producer as a
// dependent-class primary; anonymous declarations report the root conversion.
// Only project the captured direct constructor/member call in a named test.
// No diagnostics are added, removed or reordered, and Message retains the root.
func capturedReturnDependencyNativeText(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	var sources *semaSources
	for i := range diagnostics {
		d := &diagnostics[i]
		if d.Code != "GLADESEMA019" || d.Range == nil ||
			!strings.HasPrefix(d.NativeMessage, "Illegal conversion from List<String> to List<") {
			continue
		}
		for _, owner := range index.Types {
			if owner.File != d.File || !owner.HasSourceSnapshot() || owner.Dependency ||
				hasModifier(owner.Modifiers, vm.AnonymousClassModifier) {
				continue
			}
			for _, member := range owner.Members {
				if member.Kind != apexast.DeclarationMethod || !member.HasBody || len(member.Parameters) != 0 ||
					member.Range.Start.Offset > d.Range.Start.Offset || member.Range.End.Offset < d.Range.Start.Offset {
					continue
				}
				for _, caller := range index.Types {
					if !caller.IsTest || !caller.HasSourceSnapshot() || caller.Dependency ||
						hasModifier(caller.Modifiers, vm.AnonymousClassModifier) {
						continue
					}
					if sources == nil {
						sources = newSemaSources(nil, nil)
					}
					text, ok := sources.rawForType(caller)
					if !ok {
						continue
					}
					if digest, bound := index.SourceDigest(caller.File); !bound || sha256.Sum256([]byte(text)) != digest {
						continue
					}
					for _, test := range caller.Members {
						if !test.IsTest || !hasModifier(test.Modifiers, "static") || len(test.Parameters) != 0 || test.BodyRange == nil {
							continue
						}
						start, end := test.BodyRange.Start.Offset, test.BodyRange.End.Offset
						if start < 0 || end < start || end > len(text) {
							continue
						}
						program, err := vm.CompileAnonymousWithOptions(text[start:end], vm.CompileOptions{APIVersion: caller.EffectiveAPIVersion})
						if err == nil && capturedDirectReturnCall(program.Instructions, owner.Name, member.Name) {
							d.NativeMessage = fmt.Sprintf("Dependent class is invalid and needs recompilation:\n Class %s : %s", owner.Name, d.NativeMessage)
							break
						}
					}
					if strings.HasPrefix(d.NativeMessage, "Dependent class is invalid") {
						break
					}
				}
			}
		}
	}
	return diagnostics
}

func capturedDirectReturnCall(instructions []ir.Instruction, owner, member string) bool {
	var matches func(ir.Expr) bool
	matches = func(expr ir.Expr) bool {
		if expr.Kind == ir.ExprCall && strings.EqualFold(expr.Callee, member) && len(expr.Args) == 0 && expr.Left != nil {
			receiver := expr.Left
			if receiver.Kind == ir.ExprCall && strings.EqualFold(receiver.Callee, "new:"+owner) &&
				len(receiver.Args) == 0 && len(receiver.NamedArgs) == 0 {
				return true
			}
		}
		if expr.Left != nil && matches(*expr.Left) || expr.Right != nil && matches(*expr.Right) {
			return true
		}
		for _, arg := range expr.Args {
			if matches(arg) {
				return true
			}
		}
		return false
	}
	for _, inst := range instructions {
		if matches(inst.Expr) || inst.Op == ir.OpBlock && capturedDirectReturnCall(inst.Then, owner, member) {
			return true
		}
	}
	return false
}
