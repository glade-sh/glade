package vm

import (
	"strings"
	"unsafe"

	"github.com/glade-sh/glade/internal/ir"
)

// Strict mode checks the heap after every statement. The full check walks
// every local and static container, so a loop that builds a list walked the
// whole list on each statement and grew quadratically. heapCheckCache keeps
// the container part of the last check, so a statement of a few provably
// simple shapes is priced from that part instead of a new walk:
//
//   - writes to scalar locals (x = 1 + y, Integer k = i * 2, i++);
//   - list.add(v) on a local List, where v is a pure scalar expression, a new
//     record with pure named arguments, or a record local;
//   - binding a local to a new record with pure named arguments;
//   - record.Field = pure scalar expression on a record local created since
//     the last full walk.
//
// The cache is exact or unused. It is valid only from one check to the next
// statement of the same scope, with no statement and no expression evaluated
// in between (instructions and heapEvalSeq), unless a loop condition was a
// pure scalar expression (evalHeapNeutral). The statement must match its shape
// before it runs (planHeapStatement) and must not run a nested statement;
// for, while, do, if, block, break and continue statements are transparent:
// they keep the cache for their nested statements and price their own check
// when nothing but pure conditions and binding cleanup followed the last
// nested check.
// Since the last full walk only priced statements ran (the window), so a
// container whose identity is newer than the walk (windowMark) was made by
// one of them and is reachable only through the local it was bound to and
// the lists it was appended to (appended). Every other statement, and every
// plan whose conditions fail after the statement, takes the full walk.
type heapCheckCache struct {
	valid bool
	// scope is the Globals map of the check. It is held, not just its
	// address, so a later frame map cannot reuse the address.
	scope   map[string]Value
	evalSeq uint64
	// instructions is vm.work.instructions at the check.
	instructions int
	// containerTotal is the size of the container-valued locals, names
	// included, and staticTotal the static part, both from the exact check.
	containerTotal int
	staticTotal    int
	classesLen     int
	// roots holds each container-valued local at the check.
	roots map[string]heapRoot
	// windowMark is the newest value identity at the last full walk.
	windowMark uint64
	// appended holds the identity of each record local appended to a list
	// since the last full walk.
	appended map[uint64]struct{}
	// itemSeen is the empty visited set for sizing one record.
	itemSeen map[approxVisitKey]bool
	// priced counts checks priced from the cache, for tests.
	priced int
}

// heapStatementPlansOff makes every strict check walk the heap. Tests compare
// both paths with it.
var heapStatementPlansOff bool

type heapRoot struct {
	sig   heapRootSig
	value Value
}

type heapRootSig struct {
	kind ValueKind
	ref  uint64
	ptr  uintptr
	n    int
}

func heapRootSigOf(value Value) heapRootSig {
	sig := heapRootSig{kind: value.Kind, ref: value.Ref}
	switch value.Kind {
	case ValueList:
		sig.n = len(value.List)
		sig.ptr = uintptr(unsafe.Pointer(unsafe.SliceData(value.List)))
	case ValueSet:
		sig.n = len(value.Set)
		sig.ptr = uintptr(unsafe.Pointer(unsafe.SliceData(value.Set)))
	case ValueMap:
		if value.mapKeySetSource != nil {
			// A key-set view's size follows its backing map; never price it
			// from a cached walk.
			return heapRootSig{}
		}
		sig.n = len(value.Map)
		sig.ptr = heapMapAddr(value.Map)
	case ValueObject:
		sig.n = len(value.Fields)
		sig.ptr = heapMapAddr(value.Fields)
	}
	return sig
}

// heapMapAddr is the address of a map's storage, zero for a nil map.
func heapMapAddr(m map[string]Value) uintptr {
	return uintptr(*(*unsafe.Pointer)(unsafe.Pointer(&m)))
}

func heapScalarKind(kind ValueKind) bool {
	switch kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true
	}
	return false
}

type heapPlanKind uint8

const (
	heapPlanNone heapPlanKind = iota
	// heapPlanScalar: the statement writes scalar locals only.
	heapPlanScalar
	// heapPlanAppend: the statement appends one value to a local list.
	heapPlanAppend
	// heapPlanBind: the statement binds a local to a new record.
	heapPlanBind
	// heapPlanField: the statement writes a scalar field of a record local.
	heapPlanField
	// heapPlanControl: a control statement whose nested statements check
	// on their own; it evaluates only conditions, which keep the cache only
	// when pure.
	heapPlanControl
)

// heapStatementPlan is taken before a statement runs and consumed by the
// check after it.
type heapStatementPlan struct {
	kind heapPlanKind
	// target is the list, bound local or written record local.
	target string
	ref    uint64
	oldLen int
	// item names the record local an append adds, if any.
	item string
	// before is the written record's size before a field write.
	before       int
	refMark      uint64
	instructions int
}

func (vm *VM) invalidateHeapCheckCache() {
	vm.heapCheck.valid = false
	vm.heapCheck.scope = nil
}

func sameScope(a, b map[string]Value) bool {
	return a != nil && b != nil && heapMapAddr(a) == heapMapAddr(b)
}

// heapCheckCurrent reports whether the cache still describes the heap: no
// statement and no unproven expression ran since the check, in this scope.
func (vm *VM) heapCheckCurrent(instructions int) bool {
	c := &vm.heapCheck
	return c.valid && c.evalSeq == vm.heapEvalSeq && c.instructions == instructions && sameScope(c.scope, vm.Globals)
}

// planHeapStatement classifies inst before it runs. It must be called after
// the statement's instruction count was taken.
func (vm *VM) planHeapStatement(inst ir.Instruction) heapStatementPlan {
	if vm.limitMode != LimitModeStrict {
		return heapStatementPlan{}
	}
	if heapStatementPlansOff {
		vm.heapCheck.valid = false
		return heapStatementPlan{}
	}
	current := vm.heapCheckCurrent(vm.work.instructions - 1)
	if heapControlOp(inst.Op) {
		if current {
			// The statement itself writes nothing, so the cache stays
			// current for its first nested statement. break and continue
			// return before any check.
			vm.heapCheck.instructions = vm.work.instructions
		} else {
			vm.heapCheck.valid = false
		}
		return heapStatementPlan{kind: heapPlanControl}
	}
	// Nested statements check and re-arm on their own; this statement's
	// check decides from the plan, never from a stale arm.
	vm.heapCheck.valid = false
	if !current {
		return heapStatementPlan{}
	}
	plan := heapStatementPlan{instructions: vm.work.instructions, refMark: nextValueRef.Load()}
	switch inst.Op {
	case ir.OpAssign:
		if vm.heapScalarLocal(inst.Name) && vm.heapPureExpr(inst.Expr) {
			plan.kind = heapPlanScalar
		} else if heapLocalName(inst.Name) && vm.heapNewRecordExpr(inst.Expr) {
			plan.kind, plan.target = heapPlanBind, inst.Name
		} else if root, field, ok := strings.Cut(inst.Name, "."); ok && heapLocalName(root) && heapLocalName(field) && vm.heapPureExpr(inst.Expr) {
			if before, ok := vm.heapWindowRecordSize(root); ok {
				plan.kind, plan.target, plan.before = heapPlanField, root, before
				plan.ref = vm.Globals[root].Ref
			}
		}
	case ir.OpDeclare:
		if inst.Expr.Kind == "" || vm.heapPureExpr(inst.Expr) {
			plan.kind = heapPlanScalar
		} else if heapLocalName(inst.Name) && vm.heapNewRecordExpr(inst.Expr) {
			plan.kind, plan.target = heapPlanBind, inst.Name
		}
	case ir.OpExpr:
		if vm.heapIncrementExpr(inst.Expr) {
			plan.kind = heapPlanScalar
		} else if target, item, ok := vm.heapAppendTarget(inst.Expr); ok {
			list := vm.Globals[target]
			plan.kind = heapPlanAppend
			plan.target, plan.item = target, item
			plan.ref = list.Ref
			plan.oldLen = len(list.List)
		}
	}
	return plan
}

// heapControlOp reports the statements that run only conditions and nested
// statements. Their bindings come and go without an evaluation (loop
// variables, block locals), which the root comparison of the next check
// prices or refuses.
func heapControlOp(op ir.Op) bool {
	switch op {
	case ir.OpFor, ir.OpWhile, ir.OpDoWhile, ir.OpIf, ir.OpBlock, ir.OpDeclGroup, ir.OpBreak, ir.OpContinue:
		return true
	}
	return false
}

func heapLocalName(name string) bool {
	return name != "" && !strings.Contains(name, ".") && !strings.EqualFold(name, "this") && !strings.EqualFold(name, "super")
}

func (vm *VM) heapScalarLocal(name string) bool {
	if !heapLocalName(name) {
		return false
	}
	value, ok := vm.Globals[name]
	return ok && heapScalarKind(value.Kind)
}

// heapRecordLocal returns a local holding a record, not a class instance.
func (vm *VM) heapRecordLocal(name string) (Value, bool) {
	if !heapLocalName(name) {
		return Value{}, false
	}
	value, ok := vm.Globals[name]
	if !ok || value.Kind != ValueObject || value.Ref == 0 || value.classInstance || !vm.isSObjectLikeType(value.Type) {
		return Value{}, false
	}
	return value, true
}

var heapPureBinaryOperators = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true,
	"<": true, ">": true, "<=": true, ">=": true, "==": true, "!=": true,
	"&&": true, "||": true, "&": true, "|": true, "^": true,
	"<<": true, ">>": true, ">>>": true,
}

// heapPureExpr reports whether expr only reads literals and scalar locals
// through operators. Such an expression cannot run Apex code (no call, and
// scalar operands never reach equals, compareTo or toString) and writes
// nothing.
func (vm *VM) heapPureExpr(expr ir.Expr) bool {
	switch expr.Kind {
	case ir.ExprLiteral:
		return true
	case ir.ExprVariable:
		return vm.heapScalarLocal(expr.Name)
	case ir.ExprUnary:
		switch expr.Operator {
		case "-", "!", "~":
		default:
			return false
		}
		return expr.Left != nil && vm.heapPureExpr(*expr.Left)
	case ir.ExprBinary:
		if !heapPureBinaryOperators[expr.Operator] || expr.Left == nil || expr.Right == nil {
			return false
		}
		return vm.heapPureExpr(*expr.Left) && vm.heapPureExpr(*expr.Right)
	}
	return false
}

// heapNewRecordExpr matches new T(Field = pure, ...): a constructor call with
// pure named arguments only. Whether T built a record is checked on the
// result.
func (vm *VM) heapNewRecordExpr(expr ir.Expr) bool {
	if expr.Kind != ir.ExprCall || !strings.HasPrefix(expr.Callee, "new:") || len(expr.Args) != 0 || expr.Left != nil {
		return false
	}
	for _, named := range expr.NamedArgs {
		if !vm.heapPureExpr(named.Expr) {
			return false
		}
	}
	return true
}

// heapIncrementExpr matches x++, x--, ++x and --x on a scalar local.
func (vm *VM) heapIncrementExpr(expr ir.Expr) bool {
	if expr.Kind != ir.ExprCall || len(expr.Args) != 0 || len(expr.NamedArgs) != 0 {
		return false
	}
	if !strings.HasPrefix(expr.Callee, "__postfix:") && !strings.HasPrefix(expr.Callee, "__prefix:") {
		return false
	}
	return expr.Left != nil && expr.Left.Kind == ir.ExprVariable && vm.heapScalarLocal(expr.Left.Name)
}

// heapAppendTarget matches name.add(arg) on a local List with an identity,
// where arg is a pure scalar expression, a new record expression or a record
// local. It returns the list local and the record local, if any.
func (vm *VM) heapAppendTarget(expr ir.Expr) (string, string, bool) {
	if expr.Kind != ir.ExprCall || len(expr.Args) != 1 || len(expr.NamedArgs) != 0 || len(vm.currentMethod.triggerLocalMethods) != 0 {
		return "", "", false
	}
	receiverName, method := "", expr.Callee
	if expr.Left != nil {
		if expr.Left.Kind != ir.ExprVariable {
			return "", "", false
		}
		receiverName = expr.Left.Name
	} else {
		var qualified bool
		receiverName, method, qualified = strings.Cut(expr.Callee, ".")
		if !qualified {
			return "", "", false
		}
		if _, builtin := canonicalBuiltinStaticCall(expr.Callee); builtin {
			return "", "", false
		}
	}
	if !strings.EqualFold(method, "add") || !heapLocalName(receiverName) {
		return "", "", false
	}
	receiver, ok := vm.Globals[receiverName]
	if !ok || receiver.Kind != ValueList || receiver.Ref == 0 || receiver.nativeListMembership {
		return "", "", false
	}
	arg := expr.Args[0]
	if vm.heapPureExpr(arg) || vm.heapNewRecordExpr(arg) {
		return receiverName, "", true
	}
	if arg.Kind == ir.ExprVariable && arg.Name != receiverName {
		if _, ok := vm.heapRecordLocal(arg.Name); ok {
			return receiverName, arg.Name, true
		}
	}
	return "", "", false
}

// heapWindowRecordSize sizes a record local whose every container was made
// in the window, so the record is its subtree's only way in.
func (vm *VM) heapWindowRecordSize(name string) (int, bool) {
	record, ok := vm.heapRecordLocal(name)
	if !ok || !heapFreshTree(record, vm.heapCheck.windowMark, 0) {
		return 0, false
	}
	return vm.heapTreeSize(record), true
}

// heapTreeSize sizes a record subtree on its own, as the full walk counts it
// when nothing else reaches the subtree.
func (vm *VM) heapTreeSize(value Value) int {
	c := &vm.heapCheck
	if c.itemSeen == nil {
		c.itemSeen = make(map[approxVisitKey]bool, 8)
	}
	size := approxValueSizeSeen(value, c.itemSeen)
	if len(c.itemSeen) > 64 {
		c.itemSeen = nil
	} else {
		clear(c.itemSeen)
	}
	return size
}

// evalHeapNeutral evaluates a loop or if condition. A pure scalar condition
// keeps the strict heap cache current; any other expression leaves it to
// lapse.
func (vm *VM) evalHeapNeutral(expr ir.Expr, result *Result) (Value, error) {
	if vm.limitMode != LimitModeStrict || !vm.heapCheckCurrent(vm.work.instructions) || !vm.heapPureExpr(expr) {
		return vm.eval(expr, result)
	}
	instructions := vm.work.instructions
	value, err := vm.eval(expr, result)
	if err == nil && vm.work.instructions == instructions && vm.heapCheck.valid {
		vm.heapCheck.evalSeq = vm.heapEvalSeq
	}
	return value, err
}

// heapSizeFromPlan prices the heap after a planned statement from the cache.
// It returns false, leaving the cache as it was, when any condition of the
// plan does not hold.
func (vm *VM) heapSizeFromPlan(plan heapStatementPlan) (int, bool) {
	c := &vm.heapCheck
	switch {
	case plan.kind == heapPlanNone:
		return 0, false
	case plan.kind == heapPlanControl:
		// Only checks, pure conditions and binding cleanup since the last
		// nested check.
		if !vm.heapCheckCurrent(vm.work.instructions) {
			return 0, false
		}
	case vm.work.instructions != plan.instructions || !sameScope(c.scope, vm.Globals):
		return 0, false
	}
	if !vm.heapStaticFlatOK || !vm.heapStaticRootsOK || vm.heapStaticRootsLen != len(vm.Classes) || c.classesLen != len(vm.Classes) {
		return 0, false
	}
	delta := 0
	var targetSig, targetOld heapRootSig
	var appendedRef uint64
	switch plan.kind {
	case heapPlanAppend:
		target, ok := vm.Globals[plan.target]
		old, had := c.roots[plan.target]
		if !ok || !had || target.Kind != ValueList || target.Ref != plan.ref || old.sig.ref != plan.ref ||
			old.sig.n != plan.oldLen || len(target.List) != plan.oldLen+1 {
			return 0, false
		}
		item := target.List[plan.oldLen]
		if plan.item != "" {
			// The record is already counted through its local, which the
			// root comparison below requires unchanged.
			local, ok := c.roots[plan.item]
			if !ok || item.Kind != ValueObject || item.Ref != local.value.Ref {
				return 0, false
			}
			appendedRef = item.Ref
		} else {
			size, ok := vm.heapFreshItemSize(item, plan.refMark)
			if !ok {
				return 0, false
			}
			delta += size
		}
		targetSig, targetOld = heapRootSigOf(target), old.sig
	case heapPlanBind:
		bound, ok := vm.Globals[plan.target]
		if !ok || bound.Kind != ValueObject {
			return 0, false
		}
		size, ok := vm.heapFreshItemSize(bound, plan.refMark)
		if !ok {
			return 0, false
		}
		delta += size
		if old, had := c.roots[plan.target]; had {
			dropped, ok := vm.heapDroppedSize(old.value)
			if !ok {
				return 0, false
			}
			delta -= dropped
		} else {
			delta += len(plan.target)
		}
	case heapPlanField:
		record, ok := vm.heapRecordLocal(plan.target)
		if !ok || record.Ref != plan.ref {
			return 0, false
		}
		after, ok := vm.heapWindowRecordSize(plan.target)
		if !ok {
			return 0, false
		}
		delta += after - plan.before
	}
	scalarTotal, containers := 0, 0
	var changed [4]string
	changes := changed[:0]
	for name, value := range vm.Globals {
		if !heapContainerKind(value.Kind) {
			scalarTotal += len(name) + approxValueSizeSeen(value, nil)
			continue
		}
		containers++
		sig := heapRootSigOf(value)
		if sig.kind == "" {
			return 0, false
		}
		old, had := c.roots[name]
		switch {
		case plan.kind == heapPlanBind && name == plan.target:
			// Priced above.
		case !had:
			return 0, false
		case sig == old.sig && !(plan.kind == heapPlanField && name == plan.target):
			continue
		case sig == old.sig:
			// A field write in place.
		case plan.kind == heapPlanAppend && value.Ref == plan.ref && sig == targetSig && old.sig == targetOld:
			// An alias of the appended list shares its new storage.
		case plan.kind == heapPlanField && name == plan.target && value.Ref == old.value.Ref:
			// Priced above; the write may give the record new storage.
		default:
			return 0, false
		}
		changes = append(changes, name)
	}
	expected := len(c.roots)
	if plan.kind == heapPlanBind {
		if _, had := c.roots[plan.target]; !had {
			expected++
		}
	}
	var departed []string
	if containers != expected {
		// A local that left scope (a block exit) takes its name with it, and
		// its value when nothing else reaches it.
		for name, old := range c.roots {
			if _, live := vm.Globals[name]; live {
				continue
			}
			dropped, ok := vm.heapDroppedSize(old.value)
			if !ok {
				return 0, false
			}
			delta -= len(name) + dropped
			departed = append(departed, name)
		}
		if containers != expected-len(departed) {
			return 0, false
		}
	}
	// Commit.
	for _, name := range departed {
		delete(c.roots, name)
	}
	for _, name := range changes {
		value := vm.Globals[name]
		c.roots[name] = heapRoot{sig: heapRootSigOf(value), value: value}
	}
	if appendedRef != 0 {
		if c.appended == nil {
			c.appended = make(map[uint64]struct{})
		}
		c.appended[appendedRef] = struct{}{}
	}
	c.containerTotal += delta
	c.priced++
	return scalarTotal + c.containerTotal + c.staticTotal, true
}

// heapDroppedSize is what the heap loses when a local stops holding value: a
// record appended in the window to a list loses nothing, since that list
// local cannot leave or change without the check walking (a list is never
// priced as dropped), and a record made in the window and never appended
// loses its whole size. Anything else is not priced.
func (vm *VM) heapDroppedSize(value Value) (int, bool) {
	if value.Kind != ValueObject || value.Ref == 0 {
		return 0, false
	}
	if _, ok := vm.heapCheck.appended[value.Ref]; ok {
		return 0, true
	}
	if !heapFreshTree(value, vm.heapCheck.windowMark, 0) {
		return 0, false
	}
	return vm.heapTreeSize(value), true
}

// heapFreshItemSize sizes a value that cannot already be reachable: a scalar,
// or a record created during the statement. Every container in the record,
// the record included, must carry an identity newer than refMark, so no part
// of it was reachable before the statement and the full walk counts all of
// it. Records of a loaded schema hold their own bookkeeping maps, which
// qualify the same way.
func (vm *VM) heapFreshItemSize(item Value, refMark uint64) (int, bool) {
	if heapScalarKind(item.Kind) {
		return approxValueSizeSeen(item, nil), true
	}
	if item.Kind != ValueObject || item.classInstance || !vm.isSObjectLikeType(item.Type) || !heapFreshTree(item, refMark, 0) {
		return 0, false
	}
	return vm.heapTreeSize(item), true
}

// heapFreshTree reports whether every container under value has an identity
// newer than refMark and holds no shared or derived storage.
func heapFreshTree(value Value, refMark uint64, depth int) bool {
	if heapScalarKind(value.Kind) {
		return true
	}
	if !heapContainerKind(value.Kind) || depth > 4 || value.Ref <= refMark ||
		value.fieldsShared || value.mapEntriesShared || value.mapBackingShared || value.mapKeySetSource != nil {
		return false
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if !heapFreshTree(child, refMark, depth+1) {
				return false
			}
		}
	case ValueList:
		for _, child := range value.List {
			if !heapFreshTree(child, refMark, depth+1) {
				return false
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if !heapFreshTree(child, refMark, depth+1) {
				return false
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if !heapFreshTree(child, refMark, depth+1) {
				return false
			}
		}
	}
	return true
}

// heapSizeAndArm is the exact walk of currentHeapSize. It also records the
// split the cache prices later statements from and opens a new window.
func (vm *VM) heapSizeAndArm() int {
	c := &vm.heapCheck
	c.valid = false
	if c.roots == nil {
		c.roots = make(map[string]heapRoot)
	} else {
		clear(c.roots)
	}
	clear(c.appended)
	c.windowMark = nextValueRef.Load()
	seen := vm.heapSeenScratch()
	scalarTotal, containerTotal := 0, 0
	for name, value := range vm.Globals {
		size := len(name) + approxValueSizeSeen(value, seen)
		if heapContainerKind(value.Kind) {
			containerTotal += size
			c.roots[name] = heapRoot{sig: heapRootSigOf(value), value: value}
		} else {
			scalarTotal += size
		}
	}
	statics, ok := vm.staticHeapSize(seen)
	vm.releaseHeapSeen(seen)
	if !ok {
		c.scope = nil
		return vm.heapSizeOver(vm.heapStaticRootMaps())
	}
	total := scalarTotal + containerTotal + statics
	if heapVerifyAgainstFullWalk {
		vm.verifyHeapSize(total)
	}
	c.containerTotal, c.staticTotal, c.classesLen = containerTotal, statics, len(vm.Classes)
	vm.armHeapCheck()
	return total
}

func (vm *VM) armHeapCheck() {
	c := &vm.heapCheck
	c.valid = true
	c.scope = vm.Globals
	c.evalSeq = vm.heapEvalSeq
	c.instructions = vm.work.instructions
}
