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

// The captured named test reports the invalid zero-argument producer as a
// dependent-class primary; anonymous declarations report the root conversion.
// Only project the captured direct constructor/member call in a named test.
// No diagnostics are added, removed or reordered, and Message retains the root.
func capturedReturnDependencyNativeText(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	var sources *semaSources
	type source struct {
		text  string
		valid bool
	}
	validated := make(map[int]source)
	type testKey struct{ caller, member int }
	callsByTest := make(map[testKey]capturedReturnCalls)
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
				for callerIndex, caller := range index.Types {
					if !caller.IsTest || !caller.HasSourceSnapshot() || caller.Dependency ||
						hasModifier(caller.Modifiers, vm.AnonymousClassModifier) {
						continue
					}
					bodySource, checked := validated[callerIndex]
					if !checked {
						if sources == nil {
							sources = newSemaSources(nil, nil)
						}
						if text, ok := sources.rawForType(caller); ok {
							if digest, bound := index.SourceDigest(caller.File); bound && sha256.Sum256([]byte(text)) == digest {
								bodySource = source{text: text, valid: true}
							}
						}
						validated[callerIndex] = bodySource
					}
					if !bodySource.valid {
						continue
					}
					text := bodySource.text
					for testIndex, test := range caller.Members {
						if !test.IsTest || !hasModifier(test.Modifiers, "static") || len(test.Parameters) != 0 || test.BodyRange == nil {
							continue
						}
						key := testKey{caller: callerIndex, member: testIndex}
						calls, cached := callsByTest[key]
						if !cached {
							start, end := test.BodyRange.Start.Offset, test.BodyRange.End.Offset
							if start >= 0 && end >= start && end <= len(text) {
								body := text[start:end]
								// The compiler needs a dot to lower a zero-argument receiver call.
								if strings.Contains(body, ".") {
									program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: caller.EffectiveAPIVersion})
									if err == nil {
										calls = appendCapturedReturnCalls(nil, program.Instructions)
									}
								}
							}
							callsByTest[key] = calls
						}
						if calls.contains(owner.Name, member.Name) {
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

type capturedReturnCall struct{ owner, member string }

type capturedReturnCalls []capturedReturnCall

func (calls capturedReturnCalls) contains(owner, member string) bool {
	for _, call := range calls {
		if strings.EqualFold(call.owner, owner) && strings.EqualFold(call.member, member) {
			return true
		}
	}
	return false
}

func appendCapturedReturnCalls(calls capturedReturnCalls, instructions []ir.Instruction) capturedReturnCalls {
	for _, inst := range instructions {
		calls = appendCapturedReturnExprCalls(calls, inst.Expr)
		if inst.Op == ir.OpBlock {
			calls = appendCapturedReturnCalls(calls, inst.Then)
		}
	}
	return calls
}

func appendCapturedReturnExprCalls(calls capturedReturnCalls, expr ir.Expr) capturedReturnCalls {
	if expr.Kind == ir.ExprCall && len(expr.Args) == 0 && expr.Left != nil {
		receiver := expr.Left
		const prefix = "new:"
		if receiver.Kind == ir.ExprCall && len(receiver.Callee) >= len(prefix) &&
			strings.EqualFold(receiver.Callee[:len(prefix)], prefix) &&
			len(receiver.Args) == 0 && len(receiver.NamedArgs) == 0 {
			calls = append(calls, capturedReturnCall{owner: receiver.Callee[len(prefix):], member: expr.Callee})
		}
	}
	if expr.Left != nil {
		calls = appendCapturedReturnExprCalls(calls, *expr.Left)
	}
	if expr.Right != nil {
		calls = appendCapturedReturnExprCalls(calls, *expr.Right)
	}
	for _, arg := range expr.Args {
		calls = appendCapturedReturnExprCalls(calls, arg)
	}
	return calls
}
