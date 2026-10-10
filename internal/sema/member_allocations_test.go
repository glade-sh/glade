package sema

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// References preserve the pre-allocation-change selection and ordering rules.
func referenceResolveNestedTypeName(model *semaTypeMemberView, owner, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if strings.HasPrefix(strings.ToLower(owner), "system.") || strings.HasPrefix(strings.ToLower(typeName), "system.") {
		canonical := semaCanonicalPlatformAlias(typeName)
		if semaSystemBuiltinBase(canonical) {
			return canonical
		}
	}
	if semaShouldPreserveExplicitPlatformType(typeName) {
		return typeName
	}
	if strings.HasPrefix(strings.ToLower(owner), "system.") {
		candidate := "System." + typeName
		if semaExplicitPlatformQualifiedName(candidate) {
			return candidate
		}
	}
	if strings.Contains(typeName, ".") {
		// A qualified dependency/member type is already canonical when the model
		// contains that exact key. Resolve it before trying to qualify it relative
		// to the owner; otherwise a second member-model pass turns
		// pkgx.GatewayService into pkgx.pkgx.GatewayService.
		if _, ok := model.lookupName(typeName); ok {
			return typeName
		}
		if owner != "" {
			candidate := owner + "." + typeName
			if _, ok := model.lookupName(candidate); ok {
				return candidate
			}
		}
		ownerParts := strings.Split(owner, ".")
		for i := len(ownerParts) - 1; i > 0; i-- {
			candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
			if _, ok := model.lookupName(candidate); ok {
				return candidate
			}
		}
		if _, ok := model.lookupName(typeName); ok {
			return typeName
		}
		return semaCanonicalPlatformAlias(typeName)
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
	}
	if semaIsCustomAPIName(typeName) {
		if _, ok := model.lookupName(typeName); ok {
			return typeName
		}
	}
	if namespace := semaOwnerTypeNamespace(model, owner); namespace != "" {
		if namespaced, ok := semaProjectNamespacedAPIName(namespace, typeName); ok {
			if _, exists := model.lookupName(namespaced); exists {
				return namespaced
			}
		}
	}
	if owner != "" {
		candidate := owner + "." + typeName
		if _, ok := model.lookupName(candidate); ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
		if _, ok := model.lookupName(candidate); ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		enclosing := strings.Join(ownerParts[:i], ".")
		if resolved := resolveNestedTypeNameFromSuperclasses(model, enclosing, typeName); resolved != "" {
			return resolved
		}
	}
	if resolved := resolveNestedTypeNameFromSuperclasses(model, owner, typeName); resolved != "" {
		return resolved
	}
	return semaCanonicalPlatformAlias(typeName)
}

func referenceResolveNestedTypeReference(model *semaTypeMemberView, owner, typeName string) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return referenceResolveNestedTypeName(model, owner, typeName)
	}
	resolvedArgs := make([]string, len(args))
	for i, arg := range args {
		resolvedArgs[i] = referenceResolveNestedTypeReference(model, owner, arg)
	}
	return referenceResolveNestedTypeName(model, owner, base) + "<" + strings.Join(resolvedArgs, ",") + ">"
}

func referenceResolveMemberMethodsSeen(model *semaTypeMemberView, typeName, method string, seen map[string]bool) []resolvedMember {
	members, key, ok := semaLookupTypeMembers(model, typeName)
	if key == "" || seen[key] {
		return nil
	}
	seen[key] = true
	if !ok {
		return nil
	}
	resolved := make([]resolvedMember, 0)
	seenSignatures := make(map[string]bool)
	if direct := members.methods[normalizeName(method)]; len(direct) > 0 {
		for _, member := range direct {
			// Platform dispatch R115: instantiate the platform Batchable contract before
			// matching its signature to the concrete callback. Clone parameters
			// so one item type cannot change the cached platform declaration.
			if members.platform && semaDatabaseBatchableInterface(typeName) {
				member.Parameters = append([]apexast.Parameter(nil), member.Parameters...)
				member = semaInstantiateInterfaceMethod(member, typeName)
			}
			signature := methodSignatureKey(member)
			seenSignatures[signature] = true
			resolved = append(resolved, resolvedMember{owner: members.name, member: member})
		}
	}
	for _, inherited := range referenceResolveMemberMethodsSeen(model, members.superClass, method, seen) {
		signature := methodSignatureKey(inherited.member)
		if seenSignatures[signature] {
			continue
		}
		seenSignatures[signature] = true
		resolved = append(resolved, inherited)
	}
	for _, iface := range members.interfaces {
		for _, inherited := range referenceResolveMemberMethodsSeen(model, iface, method, seen) {
			signature := methodSignatureKey(inherited.member)
			if seenSignatures[signature] || semaNarrowedBatchableExecuteImplemented(model, iface, inherited, resolved) {
				continue
			}
			seenSignatures[signature] = true
			resolved = append(resolved, inherited)
		}
	}
	return resolved
}

// Reference the existing base, including native unary-null static preference.
func referenceApplicableResolvedMembers(candidates []resolvedMember, argTypes []string, model *semaTypeMemberView) []resolvedMember {
	applicable := make([]resolvedMember, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate.member, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	// These captures establish this preference for one untyped null. Other
	// call shapes keep the existing conversion/specificity rules and owners.
	if len(argTypes) != 1 || !strings.EqualFold(argTypes[0], "null") {
		return applicable
	}
	preferred := make([]resolvedMember, 0, len(applicable))
	for _, candidate := range applicable {
		hidden := false
		if hasModifier(candidate.member.Modifiers, "static") {
			for _, other := range applicable {
				if hasModifier(other.member.Modifiers, "static") &&
					!strings.EqualFold(other.owner, candidate.owner) && semaIsSubclass(model, other.owner, candidate.owner) {
					hidden = true
					break
				}
			}
		}
		if !hidden {
			preferred = append(preferred, candidate)
		}
	}
	return preferred
}

func referenceBestResolvedMemberByArgTypes(candidates []resolvedMember, argTypes []string, model *semaTypeMemberView) (resolvedMember, bool, bool) {
	applicable := referenceApplicableResolvedMembers(candidates, argTypes, model)
	// R167-R171/C010: calls containing only untyped nulls do not select the
	// most-specific user overload. Calls with typed arguments follow normal scores.
	if len(applicable) > 1 && !semaResolvedMembersAllPlatformBacked(model, applicable) {
		onlyUntypedNulls := len(argTypes) > 0
		for _, argType := range argTypes {
			onlyUntypedNulls = onlyUntypedNulls && strings.EqualFold(argType, "null")
		}
		for i, argType := range argTypes {
			if !onlyUntypedNulls || !strings.EqualFold(argType, "null") {
				continue
			}
			paramType := semaCanonicalPlatformAlias(applicable[0].member.Parameters[i].Type)
			for _, candidate := range applicable[1:] {
				if !strings.EqualFold(paramType, semaCanonicalPlatformAlias(candidate.member.Parameters[i].Type)) {
					return resolvedMember{}, false, true
				}
			}
		}
	}
	if best, ok := bestResolvedMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	if best, ok := bestResolvedMemberByConversionScore(applicable, argTypes, model); ok {
		return best, true, false
	}
	return bestResolvedMemberBySpecificity(applicable, model)
}

func referenceFilterResolvedMethodsByReceiverMode(candidates []resolvedMember, receiverMode string) []resolvedMember {
	switch receiverMode {
	case "class":
		filtered := make([]resolvedMember, 0, len(candidates))
		for _, candidate := range candidates {
			if hasModifier(candidate.member.Modifiers, "static") {
				filtered = append(filtered, candidate)
			}
		}
		return filtered
	case "instance", "super":
		filtered := make([]resolvedMember, 0, len(candidates))
		for _, candidate := range candidates {
			if !hasModifier(candidate.member.Modifiers, "static") {
				filtered = append(filtered, candidate)
			}
		}
		return filtered
	default:
		return candidates
	}
}

func allocationTestMember(name, result string, params ...string) typesys.MemberSymbol {
	member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: name, Type: result}
	for i, param := range params {
		member.Parameters = append(member.Parameters, apexast.Parameter{Name: fmt.Sprintf("arg%d", i), Type: param})
	}
	return member
}

func allocationTestView(entries ...typeMembers) *semaTypeMemberView {
	base := &semaTypeMemberModel{members: make(map[string]typeMembers)}
	for _, entry := range entries {
		base.members[normalizeName(entry.name)] = entry
	}
	return newSemaTypeMemberStateWithPlatform(base, nil).view()
}

func nestedAllocationTestView() *semaTypeMemberView {
	entries := []typeMembers{
		{name: "Root", namespace: "pkg", superClass: "Base"},
		{name: "Root.Inner", namespace: "pkg", superClass: "InnerBase"},
		{name: "Root.Inner.Leaf", namespace: "pkg"},
		{name: "Root.Target"}, {name: "Root.Inner.Target"},
		{name: "Base"}, {name: "Base.Nested"},
		{name: "InnerBase"}, {name: "InnerBase.Nested"},
		{name: "pkg__Thing__c", sobject: true},
		{name: "Root..Target"}, {name: ".Target"},
		{name: "Ångström"}, {name: "Ångström.Éclair"},
		{name: "CycleA", superClass: "CycleB"},
		{name: "CycleB", superClass: "CycleA"},
		{name: "CycleB.Target"},
	}
	return allocationTestView(entries...)
}

func TestNestedTypeAllocationChangesMatchReference(t *testing.T) {
	view := nestedAllocationTestView()
	owners := []string{"", " ", "Root", "Root.Inner", "Root.Inner.Leaf", ".Inner", "Root..Inner", "Root.Inner.", "Root...", ".", "..", " Root.Inner ", "Ångström.Inner", "CycleA", "System.HttpResponse", "SYSTEM.RestResponse", "ſystem.RestResponse", "owner\xff"}
	names := []string{"", " \t\n", "Target", " Root ", "Nested", "Thing__c", "Unknown", "Root.Target", "Missing.Target", ".Target", "Éclair", "name\xff", "System.String", "SYSTEM.Boolean", "List", "HttpResponse"}
	for _, owner := range owners {
		for _, name := range names {
			if got, want := resolveNestedTypeName(view, owner, name), referenceResolveNestedTypeName(view, owner, name); got != want {
				t.Fatalf("owner=%q name=%q: got %q, want %q", owner, name, got, want)
			}
		}
	}
	for _, typeName := range []string{"List<Target>", "Map<String,List<Nested>>", "Target[]", "Map< ,Target>", " List < Ångström.Éclair > ", "System.Map<System.String,System.List<System.Boolean>>", "List<>", "List<Unknown>", "List<name\xff>"} {
		if got, want := resolveNestedTypeReference(view, "Root.Inner.Leaf", typeName), referenceResolveNestedTypeReference(view, "Root.Inner.Leaf", typeName); got != want {
			t.Fatalf("reference=%q: got %q, want %q", typeName, got, want)
		}
	}
	// View identity is not enough to memoize names: overlays and hydration can
	// supply a formerly unresolved nested type during the same analysis.
	if got := resolveNestedTypeName(view, "Root", "Later"); got != "Later" {
		t.Fatalf("before overlay: %q", got)
	}
	view.current = map[string]typeMembers{"root.later": {name: "Root.Later"}}
	if got := resolveNestedTypeName(view, "Root", "Later"); got != "Root.Later" {
		t.Fatalf("after overlay: %q", got)
	}
	view.storeHydrated("root.hydrated", typeMembers{name: "Root.Hydrated"})
	if got := resolveNestedTypeName(view, "Root", "Hydrated"); got != "Root.Hydrated" {
		t.Fatalf("after hydration: %q", got)
	}
}

func methodsAllocationTestView() *semaTypeMemberView {
	method := func(result, param string) typesys.MemberSymbol { return allocationTestMember("run", result, param) }
	return allocationTestView(
		typeMembers{name: "Leaf", methods: map[string][]typesys.MemberSymbol{"run": {method("One", "String"), method("Two", "Integer"), method("Three", "Decimal"), method("Four", "Boolean")}}},
		typeMembers{name: "Empty"},
		typeMembers{name: "Parent", methods: map[string][]typesys.MemberSymbol{"run": {method("ParentString", "String"), method("ParentInteger", "Integer")}}},
		typeMembers{name: "Child", superClass: "Parent", interfaces: []string{"First", "Second"}, methods: map[string][]typesys.MemberSymbol{"run": {method("ChildString", "String"), method("DuplicateString", "String"), method("ChildBoolean", "Boolean")}}},
		typeMembers{name: "First", methods: map[string][]typesys.MemberSymbol{"run": {method("FirstInteger", "Integer"), method("FirstDecimal", "Decimal")}}},
		typeMembers{name: "Second", interfaces: []string{"First"}, methods: map[string][]typesys.MemberSymbol{"run": {method("SecondDecimal", "Decimal"), method("SecondLong", "Long")}}},
		typeMembers{name: "CycleA", superClass: "CycleB", methods: map[string][]typesys.MemberSymbol{"run": {method("CycleA", "String")}}},
		typeMembers{name: "CycleB", superClass: "CycleA", methods: map[string][]typesys.MemberSymbol{"run": {method("CycleB", "Integer")}}},
		typeMembers{name: "Database.Batchable", platform: true, dependency: true, methods: map[string][]typesys.MemberSymbol{"execute": {allocationTestMember("execute", "void", "Database.BatchableContext", "List<Object>")}}},
	)
}

func TestMemberResolutionAllocationChangesMatchReference(t *testing.T) {
	view := methodsAllocationTestView()
	for _, typeName := range []string{"", " ", "Missing", "Leaf", "Empty", "Child", "CycleA", "Database.Batchable<Account>", "Database.Batchable<Contact>"} {
		for _, method := range []string{"", "run", "RUN", "execute", "absent", "méthod", "run\xff"} {
			got := resolveMemberMethods(view, typeName, method)
			want := referenceResolveMemberMethodsSeen(view, typeName, method, make(map[string]bool))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("type=%q method=%q:\ngot %#v\nwant %#v", typeName, method, got, want)
			}
		}
	}
	child := resolveMemberMethods(view, "Child", "run")
	var results []string
	for _, candidate := range child {
		results = append(results, candidate.member.Type)
	}
	wantOrder := []string{"ChildString", "DuplicateString", "ChildBoolean", "ParentInteger", "FirstDecimal", "SecondLong"}
	if !reflect.DeepEqual(results, wantOrder) {
		t.Fatalf("inheritance order or direct duplicates changed: %v", results)
	}
	account := resolveMemberMethods(view, "Database.Batchable<Account>", "execute")
	contact := resolveMemberMethods(view, "Database.Batchable<Contact>", "execute")
	if len(account) != 1 || len(contact) != 1 || account[0].member.Parameters[1].Type != "List<Account>" || contact[0].member.Parameters[1].Type != "List<Contact>" {
		t.Fatalf("Batchable instantiations: account=%v contact=%v", account, contact)
	}
	base, _ := view.lookupName("Database.Batchable")
	if base.methods["execute"][0].Parameters[1].Type != "List<Object>" {
		t.Fatal("Batchable instantiation modified the platform declaration")
	}
}

func TestOverloadAllocationChangesMatchReference(t *testing.T) {
	view := allocationTestView(typeMembers{name: "User"}, typeMembers{name: "Platform", dependency: true}, typeMembers{name: "Base"}, typeMembers{name: "Derived", superClass: "Base"})
	method := func(owner, param string) resolvedMember {
		return resolvedMember{owner: owner, member: allocationTestMember("run", param, param)}
	}
	staticMethod := func(owner, param string) resolvedMember {
		candidate := method(owner, param)
		candidate.member.Modifiers = []string{"private", "static"}
		return candidate
	}
	sets := [][]resolvedMember{
		nil, {},
		{{owner: "User", member: allocationTestMember("run", "NoArgs")}},
		{method("User", "String")},
		{method("User", "Long")},
		{method("User", "Object"), method("User", "String")},
		{method("User", "String"), method("User", "String")},
		{method("Base", "String"), method("Derived", "String")},
		{method("Platform", "Object"), method("Platform", "String")},
		{method("User", "String"), method("User", "Integer"), method("User", "Long"), method("User", "Decimal"), method("User", "Object")},
		{method("User", "List<Object>"), method("User", "List<String>")},
		// Native preference must precede the single-applicable shortcut, with
		// both stack-backed and larger overload sets. Typed arguments still
		// select applicable ancestors; private derived choices stay selected.
		{staticMethod("Base", "Integer"), staticMethod("Derived", "String")},
		{staticMethod("Derived", "String"), staticMethod("Base", "Integer")},
		{staticMethod("Base", "Integer"), staticMethod("Base", "Long"), staticMethod("Base", "Decimal"), staticMethod("Base", "Object"), staticMethod("Derived", "String")},
	}
	for i, candidates := range sets {
		before := append([]resolvedMember(nil), candidates...)
		for _, args := range [][]string{nil, {""}, {"null"}, {"String"}, {"Integer"}, {"Long"}, {"void"}, {"Ångström"}, {" String "}, {"Database.QueryResult"}, {"List<String>"}, {"String", "String"}} {
			got, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, args, view)
			want, wantOK, wantAmbiguous := referenceBestResolvedMemberByArgTypes(candidates, args, view)
			if !reflect.DeepEqual(got, want) || ok != wantOK || ambiguous != wantAmbiguous {
				t.Fatalf("set=%d args=%v: got (%#v,%v,%v), want (%#v,%v,%v)", i, args, got, ok, ambiguous, want, wantOK, wantAmbiguous)
			}
		}
		if len(candidates) > 0 && !reflect.DeepEqual(candidates, before) {
			t.Fatalf("set %d: selection modified candidates", i)
		}
	}
}

func TestReceiverFilterAllocationChangesMatchReference(t *testing.T) {
	static := resolvedMember{owner: "Owner", member: allocationTestMember("staticRun", "String")}
	static.member.Modifiers = []string{"public", "STATIC"}
	instance := resolvedMember{owner: "Owner", member: allocationTestMember("instanceRun", "String")}
	for _, candidates := range [][]resolvedMember{nil, {}, {static}, {instance}, {static, static}, {instance, instance}, {static, instance, static}, {instance, static, instance}} {
		for _, mode := range []string{"", "class", "instance", "super", "implicit", "Class", " klass "} {
			got := filterResolvedMethodsByReceiverMode(candidates, mode)
			want := referenceFilterResolvedMethodsByReceiverMode(candidates, mode)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("mode=%q candidates=%v: got %#v, want %#v", mode, candidates, got, want)
			}
		}
	}
	allStatic := []resolvedMember{static, static}
	got := filterResolvedMethodsByReceiverMode(allStatic, "class")
	if &got[0] != &allStatic[0] {
		t.Fatal("all-matching filter did not reuse the input slice")
	}
	mixed := []resolvedMember{static, instance, static}
	filtered := filterResolvedMethodsByReceiverMode(mixed, "class")
	filtered[0].owner = "changed"
	if mixed[0].owner != "Owner" {
		t.Fatal("mixed filter result aliases its input")
	}
}

func TestSingleApplicableAllocationPathPreservesHydration(t *testing.T) {
	newView := func() *semaTypeMemberView {
		base := &semaTypeMemberModel{members: map[string]typeMembers{
			"account": semaStandardSObjectPlaceholder("Account"),
		}}
		platform := &semaTypeMemberModel{members: map[string]typeMembers{
			"remoteparent": {name: "RemoteParent", dependency: true},
			"remotechild":  {name: "RemoteChild", dependency: true, superClass: "RemoteParent"},
		}}
		return newSemaTypeMemberStateWithPlatform(base, platform).view()
	}
	for _, pair := range [][2]string{{"List<SObject>", "List<Account>"}, {"RemoteParent", "RemoteChild"}} {
		for _, extraInapplicable := range []bool{false, true} {
			candidates := []resolvedMember{{owner: "Owner", member: allocationTestMember("run", "Result", pair[0])}}
			if extraInapplicable {
				candidates = append(candidates, resolvedMember{owner: "Owner", member: allocationTestMember("run", "Other", "String")})
			}
			gotView, wantView := newView(), newView()
			got, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, []string{pair[1]}, gotView)
			want, wantOK, wantAmbiguous := referenceBestResolvedMemberByArgTypes(candidates, []string{pair[1]}, wantView)
			if !reflect.DeepEqual(got, want) || ok != wantOK || ambiguous != wantAmbiguous {
				t.Fatalf("pair=%v extra=%v: selected member or flags changed", pair, extraInapplicable)
			}
			if len(wantView.hydrated) == 0 {
				t.Fatalf("pair=%v did not exercise hydration", pair)
			}
			if !reflect.DeepEqual(gotView.hydrated, wantView.hydrated) || !reflect.DeepEqual(gotView.current, wantView.current) || !reflect.DeepEqual(gotView.state.base.members, wantView.state.base.members) {
				t.Fatalf("pair=%v extra=%v: model state differs after selection", pair, extraInapplicable)
			}
		}
	}
}

var memberAllocationSliceSink []resolvedMember
var memberAllocationResultSink resolvedMember
var memberAllocationNameSink string

func TestMemberAllocationFastPaths(t *testing.T) {
	candidates := []resolvedMember{{owner: "Owner", member: allocationTestMember("run", "String")}}
	if got := testing.AllocsPerRun(100, func() { memberAllocationSliceSink = filterResolvedMethodsByReceiverMode(candidates, "instance") }); got != 0 {
		t.Fatalf("all-matching receiver filter allocated %g times", got)
	}
	if got := testing.AllocsPerRun(100, func() { memberAllocationResultSink, _, _ = bestResolvedMemberByArgTypes(candidates, nil, nil) }); got != 0 {
		t.Fatalf("single no-argument candidate allocated %g times", got)
	}
	view := nestedAllocationTestView()
	before := testing.AllocsPerRun(100, func() { memberAllocationNameSink = referenceResolveNestedTypeName(view, "Root.Inner.Leaf", "Unknown") })
	after := testing.AllocsPerRun(100, func() { memberAllocationNameSink = resolveNestedTypeName(view, "Root.Inner.Leaf", "Unknown") })
	if after >= before {
		t.Fatalf("nested lookup allocations did not fall: before=%g after=%g", before, after)
	}
	methods := methodsAllocationTestView()
	before = testing.AllocsPerRun(100, func() {
		memberAllocationSliceSink = referenceResolveMemberMethodsSeen(methods, "Leaf", "run", make(map[string]bool))
	})
	after = testing.AllocsPerRun(100, func() { memberAllocationSliceSink = resolveMemberMethods(methods, "Leaf", "run") })
	if after >= before {
		t.Fatalf("leaf method allocations did not fall: before=%g after=%g", before, after)
	}
}

func BenchmarkMemberAllocationChanges(b *testing.B) {
	view := nestedAllocationTestView()
	methods := methodsAllocationTestView()
	candidates := []resolvedMember{{owner: "Owner", member: allocationTestMember("run", "String")}}
	multiple := []resolvedMember{{owner: "Owner", member: allocationTestMember("run", "String", "String")}, {owner: "Owner", member: allocationTestMember("run", "Object", "Object")}}
	multipleArgs := []string{"String"}
	for _, operation := range []string{"nested", "methods", "overload", "overload_multiple", "filter"} {
		for _, optimized := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/optimized=%v", operation, optimized), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					switch operation {
					case "nested":
						if optimized {
							memberAllocationNameSink = resolveNestedTypeName(view, "Root.Inner.Leaf", "Unknown")
						} else {
							memberAllocationNameSink = referenceResolveNestedTypeName(view, "Root.Inner.Leaf", "Unknown")
						}
					case "methods":
						if optimized {
							memberAllocationSliceSink = resolveMemberMethods(methods, "Leaf", "run")
						} else {
							memberAllocationSliceSink = referenceResolveMemberMethodsSeen(methods, "Leaf", "run", make(map[string]bool))
						}
					case "overload":
						if optimized {
							memberAllocationResultSink, _, _ = bestResolvedMemberByArgTypes(candidates, nil, nil)
						} else {
							memberAllocationResultSink, _, _ = referenceBestResolvedMemberByArgTypes(candidates, nil, nil)
						}
					case "overload_multiple":
						if optimized {
							memberAllocationResultSink, _, _ = bestResolvedMemberByArgTypes(multiple, multipleArgs, nil)
						} else {
							memberAllocationResultSink, _, _ = referenceBestResolvedMemberByArgTypes(multiple, multipleArgs, nil)
						}
					case "filter":
						if optimized {
							memberAllocationSliceSink = filterResolvedMethodsByReceiverMode(candidates, "instance")
						} else {
							memberAllocationSliceSink = referenceFilterResolvedMethodsByReceiverMode(candidates, "instance")
						}
					}
				}
			})
		}
	}
}
