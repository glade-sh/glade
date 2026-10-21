package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// RuntimeExpressionTypeResolver retains the analyzer's expression typing for
// lazy runtime branches. Build the member view only if a runtime needs it, once
// per source generation. Cloned VMs can share it safely; model inference caches
// are protected by the mutex. Anonymous/local-only declarations fall back to
// the VM's registered declarations when they are outside this index.
func RuntimeExpressionTypeResolver(index typesys.Index) vm.ExpressionTypeResolver {
	var mu sync.Mutex
	var analyzer *Analyzer
	var model *semaTypeMemberView
	var owners map[string]typesys.TypeSymbol
	return func(expr ir.Expr, variables map[string]string, owner string) string {
		if owner == "" {
			return ""
		}
		mu.Lock()
		defer mu.Unlock()
		if owners == nil {
			owners = make(map[string]typesys.TypeSymbol, len(index.Types))
			addOwner := func(name string, candidate typesys.TypeSymbol) {
				key := normalizeName(name)
				if existing, found := owners[key]; found && (existing.File != candidate.File || existing.Namespace != candidate.Namespace || existing.Name != candidate.Name) {
					// A bare alias shared by project and dependency classes is
					// not enough to identify a semantic source occurrence.
					owners[key] = typesys.TypeSymbol{}
					return
				}
				owners[key] = candidate
			}
			for _, candidate := range index.Types {
				addOwner(candidate.Name, candidate)
				if candidate.Namespace != "" && !strings.HasPrefix(strings.ToLower(candidate.Name), strings.ToLower(candidate.Namespace)+".") {
					addOwner(candidate.Namespace+"."+candidate.Name, candidate)
				}
			}
		}
		typ, found := owners[normalizeName(owner)]
		if !found || typ.Name == "" {
			return ""
		}
		if model == nil {
			analyzer = NewAnalyzer()
			model = buildSemaTypeMemberView(index)
		}
		bodyModel := model
		if semaSourceUnitHasDuplicate(model, typ) {
			bodyModel = semaModelWithCurrentType(model, typ)
		}
		bindings := semaBodyBaseScope(typ, typesys.MemberSymbol{}, bodyModel)
		for name, typeName := range variables {
			bindings[normalizeName(name)] = typeName
		}
		scope := newIRSemaScope(bindings)
		return resolveNestedTypeReference(bodyModel, typ.Name, analyzer.inferIRExprType(expr, scope, bodyModel, typ.Name))
	}
}
