package apextest

import (
	"reflect"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/vm"
)

const runtimeStructuralMaxDepth = 256

const runtimeInlineExprVisits = 64

type runtimeExprValidationState struct {
	inline   [runtimeInlineExprVisits]*ir.Expr
	overflow map[*ir.Expr]bool
	count    int
}

func (s *runtimeExprValidationState) enter(expr *ir.Expr) bool {
	inlineCount := min(s.count, len(s.inline))
	for _, active := range s.inline[:inlineCount] {
		if active == expr {
			return false
		}
	}
	if s.count >= len(s.inline) && s.overflow[expr] {
		return false
	}
	if s.count < len(s.inline) {
		s.inline[s.count] = expr
	} else {
		if s.overflow == nil {
			s.overflow = make(map[*ir.Expr]bool)
		}
		s.overflow[expr] = true
	}
	s.count++
	return true
}

func (s *runtimeExprValidationState) leave(expr *ir.Expr) {
	s.count--
	if s.count < len(s.inline) {
		s.inline[s.count] = nil
		return
	}
	delete(s.overflow, expr)
}

type runtimeValueValidationState struct {
	active [runtimeStructuralMaxDepth + 1]runtimePatchValueContainerIdentity
	count  int
}

func (s *runtimeValueValidationState) enter(identity runtimePatchValueContainerIdentity) bool {
	for _, active := range s.active[:s.count] {
		if active == identity {
			return false
		}
	}
	if s.count == len(s.active) {
		return false
	}
	s.active[s.count] = identity
	s.count++
	return true
}

func (s *runtimeValueValidationState) leave() {
	s.count--
	s.active[s.count] = runtimePatchValueContainerIdentity{}
}

func validateRuntimeCacheEntryStructure(entry runtimeCacheEntry) bool {
	var exprState runtimeExprValidationState
	if !entry.restored.Valid() || !runtimePatchMethodsStructurallyValid(entry.Methods, &exprState) {
		return false
	}
	for _, class := range entry.Classes {
		if !runtimePatchClassStructurallyValid(class, &exprState) {
			return false
		}
	}
	for _, trigger := range entry.Triggers {
		if !runtimePatchProgramStructurallyValid(trigger.Program, &exprState, 0) {
			return false
		}
	}
	return true
}

func runtimePatchClassStructurallyValid(class vm.Class, exprState *runtimeExprValidationState) bool {
	if !runtimePatchFieldsStructurallyValid(class.Fields, exprState) ||
		!runtimePatchFieldsStructurallyValid(class.StaticFields, exprState) ||
		!runtimePatchMethodsStructurallyValid(class.Methods, exprState) {
		return false
	}
	for _, method := range class.Constructors {
		if !runtimePatchMethodStructurallyValid(method, exprState) {
			return false
		}
	}
	for _, method := range class.StaticInitializers {
		if !runtimePatchMethodStructurallyValid(method, exprState) {
			return false
		}
	}
	for _, method := range class.InstanceInitializers {
		if !runtimePatchMethodStructurallyValid(method, exprState) {
			return false
		}
	}
	return true
}

func runtimePatchFieldsStructurallyValid(fields map[string]vm.Field, exprState *runtimeExprValidationState) bool {
	for _, field := range fields {
		if !runtimePatchValueStructurallyValid(field.Value) ||
			!runtimePatchValueStructurallyValid(field.InitialValue) {
			return false
		}
		if field.Getter != nil && !runtimePatchMethodStructurallyValid(*field.Getter, exprState) {
			return false
		}
		if field.Setter != nil && !runtimePatchMethodStructurallyValid(*field.Setter, exprState) {
			return false
		}
	}
	return true
}

func runtimePatchMethodsStructurallyValid(methods map[string]vm.Method, exprState *runtimeExprValidationState) bool {
	for _, method := range methods {
		if !runtimePatchMethodStructurallyValid(method, exprState) {
			return false
		}
	}
	return true
}

func runtimePatchMethodStructurallyValid(method vm.Method, exprState *runtimeExprValidationState) bool {
	return runtimePatchProgramStructurallyValid(method.Program, exprState, 0)
}

func runtimePatchProgramStructurallyValid(program ir.Program, state *runtimeExprValidationState, depth int) bool {
	return runtimePatchInstructionsStructurallyValid(program.Instructions, state, depth+1)
}

func runtimePatchInstructionsStructurallyValid(instructions []ir.Instruction, state *runtimeExprValidationState, depth int) bool {
	if depth > runtimeStructuralMaxDepth {
		return false
	}
	for _, instruction := range instructions {
		if !runtimePatchInstructionStructurallyValid(instruction, state, depth+1) {
			return false
		}
	}
	return true
}

func runtimePatchInstructionStructurallyValid(instruction ir.Instruction, state *runtimeExprValidationState, depth int) bool {
	if depth > runtimeStructuralMaxDepth ||
		!runtimePatchExprStructurallyValid(instruction.Expr, state, depth+1) {
		return false
	}
	if instruction.Init != nil && !runtimePatchInstructionStructurallyValid(*instruction.Init, state, depth+1) {
		return false
	}
	if instruction.Update != nil && !runtimePatchInstructionStructurallyValid(*instruction.Update, state, depth+1) {
		return false
	}
	for _, nested := range [][]ir.Instruction{
		instruction.Inits,
		instruction.Updates,
		instruction.Then,
		instruction.Else,
		instruction.Catch,
		instruction.Finally,
	} {
		if !runtimePatchInstructionsStructurallyValid(nested, state, depth+1) {
			return false
		}
	}
	for _, clause := range instruction.Catches {
		if !runtimePatchInstructionsStructurallyValid(clause.Body, state, depth+1) {
			return false
		}
	}
	for _, switchCase := range instruction.Cases {
		for _, expr := range switchCase.Exprs {
			if !runtimePatchExprStructurallyValid(expr, state, depth+1) {
				return false
			}
		}
		if !runtimePatchInstructionsStructurallyValid(switchCase.Body, state, depth+1) {
			return false
		}
	}
	return true
}

func runtimePatchExprStructurallyValid(expr ir.Expr, state *runtimeExprValidationState, depth int) bool {
	if depth > runtimeStructuralMaxDepth {
		return false
	}
	for _, arg := range expr.Args {
		if !runtimePatchExprStructurallyValid(arg, state, depth+1) {
			return false
		}
	}
	for _, arg := range expr.NamedArgs {
		if !runtimePatchExprStructurallyValid(arg.Expr, state, depth+1) {
			return false
		}
	}
	for _, nested := range []*ir.Expr{expr.Left, expr.Right} {
		if nested == nil {
			continue
		}
		if !state.enter(nested) {
			return false
		}
		valid := runtimePatchExprStructurallyValid(*nested, state, depth+1)
		state.leave(nested)
		if !valid {
			return false
		}
	}
	return true
}

func runtimePatchValueStructurallyValid(value vm.Value) bool {
	var state runtimeValueValidationState
	return runtimePatchValueStructurallyValidAtDepth(value, &state, 0)
}

func runtimePatchValueStructurallyValidAtDepth(value vm.Value, state *runtimeValueValidationState, depth int) bool {
	if depth > runtimeStructuralMaxDepth ||
		!runtimePatchValueMapStructurallyValid('f', value.Fields, state, depth+1) ||
		!runtimePatchValueSliceStructurallyValid('l', value.List, state, depth+1) ||
		!runtimePatchValueSliceStructurallyValid('s', value.Set, state, depth+1) ||
		!runtimePatchValueMapStructurallyValid('m', value.Map, state, depth+1) ||
		!runtimePatchValueMapStructurallyValid('k', value.MapKeys, state, depth+1) {
		return false
	}
	return true
}

func runtimePatchValueMapStructurallyValid(kind byte, values map[string]vm.Value, state *runtimeValueValidationState, depth int) bool {
	if values == nil {
		return true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if !state.enter(identity) {
		return false
	}
	defer state.leave()
	for _, value := range values {
		if !runtimePatchValueStructurallyValidAtDepth(value, state, depth+1) {
			return false
		}
	}
	return true
}

func runtimePatchValueSliceStructurallyValid(kind byte, values []vm.Value, state *runtimeValueValidationState, depth int) bool {
	if values == nil {
		return true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if !state.enter(identity) {
		return false
	}
	defer state.leave()
	for _, value := range values {
		if !runtimePatchValueStructurallyValidAtDepth(value, state, depth+1) {
			return false
		}
	}
	return true
}
