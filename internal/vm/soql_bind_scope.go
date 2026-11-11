package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// Dynamic SOQL binds (Database.query) resolve a String local by name at run
// time. testdata/conformance/loop_scope_dynamic_binds (62.0 and 67.0,
// anonymous and named-class routes) shows what Salesforce keeps visible:
//
//   - A String local whose block has ended, by any exit (end, break, continue,
//     throw), still binds with its last value. Entering the block again, or a
//     block that holds it, hides it until its declaration runs again.
//   - Such a local outranks a class field. In anonymous Apex it also outranks
//     a same-named top-level local; a named class rejects that redeclaration,
//     and a method local declared after the ended block wins. A local that a
//     later block declares, once that declaration has run, outranks the ended
//     one.
//   - A foreach variable binds as null after its loop.
//   - Anonymous Apex only: a String declared later at the top level binds as
//     null before its declaration runs.
//
// A bindSlot is one ended local. Only String locals are tracked; other types
// keep the closedSOQLLocals behavior in soql_bind_closed_local.go.
type bindSlot struct {
	value Value
	// site is the Pos of the declaration (or foreach/for statement) the local
	// came from; entering a block drops the slots whose site lies inside it.
	site int
	// liveShadow is set when a block that declares the same name starts while
	// no local of that name is live; once that declaration runs, the live
	// local wins over this slot.
	liveShadow bool
}

// bindFrameState is per call frame; callers save and restore it with
// closedSOQLLocals.
type bindFrameState struct {
	slots map[string][]bindSlot
	// declWatch counts declarations that ran per lower-case name. It exists
	// only after a block redeclared a live name (anonymous Apex), so the
	// common path never touches it.
	declWatch map[string]int
	// anonymous marks the top-level frame of an anonymous block, and
	// anonymousBody is its top-level body: later-declared String locals bind
	// as null there.
	anonymous     bool
	anonymousBody []ir.Instruction
}

func isStringBindType(typeName string) bool {
	return strings.EqualFold(canonicalRuntimePlatformType(typeName), "String")
}

// watchDecl starts counting declarations of name and returns the count so far.
func (f *bindFrameState) watchDecl(name string) int {
	if f.declWatch == nil {
		f.declWatch = make(map[string]int)
	}
	return f.declWatch[strings.ToLower(name)]
}

func (f *bindFrameState) noteDeclared(name string) {
	f.declWatch[strings.ToLower(name)]++
}

func (f *bindFrameState) declaredSince(name string, start int) bool {
	return f.declWatch[strings.ToLower(name)] > start
}

// rememberDepartedString records the last value of a String local whose block
// ended. It also keeps closedSOQLLocals, which the other dynamic routes read,
// in step.
func (vm *VM) rememberDepartedString(name string, site int, value Value) {
	if value.Kind != ValueString && value.Kind != ValueNull {
		return
	}
	key := strings.ToLower(name)
	f := &vm.bindFrame
	if f.slots == nil {
		f.slots = make(map[string][]bindSlot)
	}
	slots := f.slots[key]
	for i := range slots {
		if slots[i].site == site {
			slots = append(slots[:i], slots[i+1:]...)
			break
		}
	}
	f.slots[key] = append(slots, bindSlot{value: value, site: site})
	if vm.closedSOQLLocals == nil {
		vm.closedSOQLLocals = make(map[string]Value)
	}
	vm.closedSOQLLocals[key] = value
}

// enterBindBlock runs when a block body starts. It drops the ended locals the
// body (or a block inside it) declared, and marks ended locals that a
// declaration in this body will outrank.
func (vm *VM) enterBindBlock(body []ir.Instruction, declared func(func(name string, live bool))) {
	f := &vm.bindFrame
	if len(f.slots) == 0 || len(body) == 0 {
		return
	}
	inside := vm.bindBodySites(body)
	for key, slots := range f.slots {
		kept := slots[:0]
		for _, slot := range slots {
			if _, in := inside[slot.site]; !in {
				kept = append(kept, slot)
			}
		}
		switch {
		case len(kept) == len(slots):
		case len(kept) == 0:
			delete(f.slots, key)
			vm.dropClosedString(key)
		default:
			f.slots[key] = kept
			vm.closedSOQLLocals[key] = kept[len(kept)-1].value
		}
	}
	if len(f.slots) == 0 {
		return
	}
	declared(func(name string, live bool) {
		if live {
			return
		}
		slots := f.slots[strings.ToLower(name)]
		for i := range slots {
			slots[i].liveShadow = true
		}
	})
}

func (vm *VM) dropClosedString(key string) {
	if value, ok := vm.closedSOQLLocals[key]; ok && (value.Kind == ValueString || value.Kind == ValueNull) {
		delete(vm.closedSOQLLocals, key)
	}
}

// departedBind is the Database.query lookup for a name that has ended slots.
func (vm *VM) departedBind(name string) (Value, bool) {
	if len(vm.bindFrame.slots) == 0 {
		return Null, false
	}
	slots := vm.bindFrame.slots[strings.ToLower(name)]
	if len(slots) == 0 {
		return Null, false
	}
	if !vm.bindFrame.anonymous {
		// In a named class a live local of the method is declared after any
		// ended local of its name, and it wins. Fields do not.
		if _, live := vm.Globals[name]; live {
			return Null, false
		}
		if _, live := vm.lookupGlobalName(name); live {
			return Null, false
		}
	}
	last := slots[len(slots)-1]
	if last.liveShadow {
		if value, err := vm.lookup(name); err == nil {
			return value, true
		}
	}
	return last.value, true
}

// hoistedAnonymousString reports a String declared at the top level of the
// running anonymous block, which has not run yet.
func (vm *VM) hoistedAnonymousString(name string) bool {
	var declares func([]ir.Instruction) bool
	declares = func(instructions []ir.Instruction) bool {
		for i := range instructions {
			switch inst := &instructions[i]; inst.Op {
			case ir.OpDeclare:
				if strings.EqualFold(inst.Name, name) && isStringBindType(inst.Type) {
					return true
				}
			case ir.OpDeclGroup:
				if declares(inst.Then) {
					return true
				}
			}
		}
		return false
	}
	return declares(vm.bindFrame.anonymousBody)
}

const maxBindBodySiteCache = 512

// bindBodySites returns the declaration sites inside a block body, cached by
// the identity of its first instruction. The cache is bounded.
func (vm *VM) bindBodySites(body []ir.Instruction) map[int]struct{} {
	head := &body[0]
	if sites, ok := vm.bindBodySiteCache[head]; ok {
		return sites
	}
	sites := make(map[int]struct{})
	collectBindSites(body, sites)
	if vm.bindBodySiteCache == nil || len(vm.bindBodySiteCache) >= maxBindBodySiteCache {
		vm.bindBodySiteCache = make(map[*ir.Instruction]map[int]struct{})
	}
	vm.bindBodySiteCache[head] = sites
	return sites
}

func collectBindSites(instructions []ir.Instruction, sites map[int]struct{}) {
	for i := range instructions {
		inst := &instructions[i]
		if inst.Op == ir.OpDeclare || inst.Op == ir.OpForEach {
			sites[inst.Pos] = struct{}{}
		}
		if inst.Init != nil {
			collectBindSites([]ir.Instruction{*inst.Init}, sites)
		}
		collectBindSites(inst.Inits, sites)
		collectBindSites(inst.Then, sites)
		collectBindSites(inst.Else, sites)
		collectBindSites(inst.Catch, sites)
		collectBindSites(inst.Finally, sites)
		for _, clause := range inst.Catches {
			collectBindSites(clause.Body, sites)
		}
		for _, c := range inst.Cases {
			collectBindSites(c.Body, sites)
		}
	}
}
