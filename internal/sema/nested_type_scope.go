package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/typesys"
)

// scopedNestedType resolves a bare name to the nested source type it denotes
// inside owner: nested types of the owner and of its superclasses first, then
// those of each enclosing type and its superclasses. Native rows
// (standard_object_shape_compile H07, I01-I16): a subclass sees the nested
// types of its superclass by their bare names.
func (a *Analyzer) scopedNestedType(model *semaTypeMemberView, owner, short string) (typeMembers, bool) {
	short = strings.TrimSpace(short)
	if model == nil || owner == "" || short == "" || strings.ContainsAny(short, ".<>[](), ") {
		return typeMembers{}, false
	}
	for scope := strings.TrimSpace(owner); scope != ""; {
		cls := scope
		for depth := 0; cls != "" && depth < 16; depth++ {
			if members, known := model.lookupName(cls + "." + short); known && !members.platform {
				if !members.dependency {
					return members, true
				}
				if caller, found := model.lookupName(owner); found && members.receiverDependencySource && caller.namespace != "" && strings.EqualFold(caller.namespace, members.namespace) {
					return members, true
				}
			}
			cls = a.superClassOf(model, cls, scope)
		}
		dot := strings.LastIndexByte(scope, '.')
		if dot < 0 {
			break
		}
		scope = scope[:dot]
	}
	return typeMembers{}, false
}

// ownerInheritsSource reports that owner, or a type enclosing it, extends a
// class; without a superclass a bare name cannot reach an inherited nested type.
func (a *Analyzer) ownerInheritsSource(model *semaTypeMemberView, owner typesys.TypeSymbol) bool {
	for scope := strings.TrimSpace(owner.Name); scope != ""; {
		if members, known := model.lookupName(scope); known && members.superClass != "" {
			return true
		}
		dot := strings.LastIndexByte(scope, '.')
		if dot < 0 {
			break
		}
		scope = scope[:dot]
	}
	return false
}

// superClassOf names the superclass of cls as a key the model knows, or "".
func (a *Analyzer) superClassOf(model *semaTypeMemberView, cls, scope string) string {
	members, known := model.lookupName(cls)
	if !known || members.superClass == "" {
		return ""
	}
	super := strings.TrimSpace(members.superClass)
	if root, rest, ok := strings.Cut(super, "."); ok && a.namespace != "" && strings.EqualFold(root, a.namespace) {
		super = rest
	}
	if i := strings.IndexByte(super, '<'); i >= 0 {
		super = super[:i]
	}
	if _, found := model.lookupName(super); found {
		return super
	}
	for outer := scope; outer != ""; {
		if dot := strings.LastIndexByte(outer, '.'); dot >= 0 {
			outer = outer[:dot]
		} else {
			outer = ""
		}
		if outer == "" {
			break
		}
		if _, found := model.lookupName(outer + "." + super); found {
			return outer + "." + super
		}
	}
	return ""
}
