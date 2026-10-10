package sema

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestGeneratedPlatformSignatureMatchesMaterializedCatalog(t *testing.T) {
	platform := buildSemaPlatformTypeMemberModel()
	state := newSemaTypeMemberStateWithPlatform(nil, platform)
	gotView, wantView := state.view(), state.view()
	keys := make([]string, 0, len(platform.platform.symbolsByKey))
	for key := range platform.platform.symbolsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	checked := 0
	seenReceivers := make(map[string]bool)
	for _, key := range keys {
		symbol := platform.platform.symbolsByKey[key]
		methods := map[string]bool{"__missing_signature__": true}
		for _, member := range symbol.Members {
			if member.Kind == apexast.DeclarationMethod {
				methods[member.Name] = true
			}
		}
		// Include both the catalog spelling and each qualified/short alias.
		for _, receiver := range []string{symbol.Name, key} {
			if seenReceivers[receiver] || !semaKnownPlatformTypeReceiver(receiver) {
				continue
			}
			seenReceivers[receiver] = true
			for method := range methods {
				for _, mode := range []string{"class", "instance", "super", "implicit"} {
					got, gotOK := semaGeneratedPlatformMethodSignature(gotView, receiver, method, mode)
					want, wantOK := materializedPlatformSignatureForTest(wantView, receiver, method, mode)
					if gotOK != wantOK || !reflect.DeepEqual(got, want) {
						t.Fatalf("%s.%s (%s): got %#v, %v; want %#v, %v", receiver, method, mode, got, gotOK, want, wantOK)
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no platform signatures checked")
	}
}

func TestGeneratedPlatformSignaturePreservesOrderFallbackAndIsolation(t *testing.T) {
	symbols := []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "Database", SuperClass: "PlatformBase", Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "Read", Type: "Id"},
			{Kind: apexast.DeclarationMethod, Name: "Read", Type: "String", Modifiers: []string{"static"}, Parameters: []apexast.Parameter{{Type: "String"}}},
			{Kind: apexast.DeclarationMethod, Name: "read", Type: "Id", Modifiers: []string{"static"}, Parameters: []apexast.Parameter{{Type: "String"}}},
			{Kind: apexast.DeclarationMethod, Name: "Read", Type: "Boolean", Modifiers: []string{"static"}, Parameters: []apexast.Parameter{{Type: "Integer"}}},
			{Kind: apexast.DeclarationMethod, Name: "Read", Type: "Object", Parameters: []apexast.Parameter{{Type: "Boolean"}}},
			{Kind: apexast.DeclarationConstructor, Name: "Read", Parameters: []apexast.Parameter{{Type: "Id"}}},
		}},
		{Kind: apexast.DeclarationClass, Name: "PlatformBase", Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "fallback", Type: "Integer", Modifiers: []string{"static"}},
		}},
	}
	platform := &semaTypeMemberModel{platform: newSemaLazyPlatformTypeMemberModel(symbols)}
	state := newSemaTypeMemberStateWithPlatform(nil, platform)
	view := state.view()
	for _, tc := range []struct {
		method, mode string
		want         semaCollectionSignature
	}{
		{"READ", "class", semaCollectionSignature{returnType: "String", params: [][]string{{"String"}, {"Integer"}}}},
		{"Read", "instance", semaCollectionSignature{returnType: "Object", params: [][]string{{"Boolean"}}}},
		{"fallback", "class", semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}},
	} {
		got, ok := semaGeneratedPlatformMethodSignature(view, "Database", tc.method, tc.mode)
		if !ok || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s (%s): got %#v, %v; want %#v", tc.method, tc.mode, got, ok, tc.want)
		}
		if len(got.params) > 0 && len(got.params[0]) > 0 {
			got.params[0][0] = "CallerMutation"
			again, ok := semaGeneratedPlatformMethodSignature(view, "Database", tc.method, tc.mode)
			if !ok || !reflect.DeepEqual(again, tc.want) {
				t.Fatalf("signature or catalog retained caller mutation: %#v, %v", again, ok)
			}
		}
	}
	if symbols[0].Members[1].Parameters[0].Type != "String" {
		t.Fatal("signature extraction mutated the platform catalog")
	}
	for _, receiver := range []string{"Database", "database", "System.Database", "MissingType", ""} {
		for _, mode := range []string{"class", "instance", "implicit"} {
			gotView, wantView := state.view(), state.view()
			// A local type with a platform name must keep the old ownership guard.
			shadow := typeMembers{name: "Database", kind: apexast.DeclarationClass, methods: map[string][]typesys.MemberSymbol{
				"read": {{Kind: apexast.DeclarationMethod, Name: "Read", Type: "Decimal", Modifiers: []string{"static"}}},
			}}
			gotView.current = map[string]typeMembers{"database": shadow}
			wantView.current = map[string]typeMembers{"database": shadow}
			got, gotOK := semaGeneratedPlatformMethodSignature(gotView, receiver, "Read", mode)
			want, wantOK := materializedPlatformSignatureForTest(wantView, receiver, "Read", mode)
			if gotOK != wantOK || !reflect.DeepEqual(got, want) {
				t.Fatalf("shadowed %s.Read (%s): got %#v, %v; want %#v, %v", receiver, mode, got, gotOK, want, wantOK)
			}
		}
	}
}

func BenchmarkGeneratedPlatformSignatureLookup(b *testing.B) {
	for _, tc := range []struct{ receiver, method, mode string }{
		{"Database", "query", "class"},
		{"Schema.DescribeSObjectResult", "getName", "instance"},
		{"Location", "newInstance", "class"},
		{"System.Location", "newInstance", "class"},
	} {
		b.Run(tc.receiver+"."+tc.method, func(b *testing.B) {
			for _, implementation := range []struct {
				name string
				call func(*semaTypeMemberView, string, string, string) (semaCollectionSignature, bool)
			}{
				{"filtered", semaGeneratedPlatformMethodSignature},
				{"materialized", materializedPlatformSignatureForTest},
			} {
				b.Run(implementation.name, func(b *testing.B) {
					view := newSemaTypeMemberStateWithPlatform(nil, semaPlatformTypeMemberModel()).view()
					want, wantOK := materializedPlatformSignatureForTest(view, tc.receiver, tc.method, tc.mode)
					if !wantOK {
						b.Fatal("benchmark signature did not resolve")
					}
					b.ReportAllocs()
					b.ResetTimer()
					var got semaCollectionSignature
					var ok bool
					for i := 0; i < b.N; i++ {
						got, ok = implementation.call(view, tc.receiver, tc.method, tc.mode)
					}
					b.StopTimer()
					if ok != wantOK || !reflect.DeepEqual(got, want) {
						b.Fatalf("signature changed: %#v, %v; want %#v, %v", got, ok, want, wantOK)
					}
				})
			}
		})
	}
}

// materializedPlatformSignatureForTest retains the pre-optimization path as an
// equivalence oracle and benchmark baseline, including its full catalog clone.
func materializedPlatformSignatureForTest(model *semaTypeMemberView, receiverType, method, receiverMode string) (semaCollectionSignature, bool) {
	if strings.TrimSpace(receiverType) == "" || strings.TrimSpace(method) == "" {
		return semaCollectionSignature{}, false
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if semaKnownPlatformTypeReceiver(receiverType) && model != nil && model.state != nil && model.state.platform != nil && model.state.platform.platform != nil {
		if platformSymbol, ok := model.state.platform.platform.symbolsByKey[normalizeName(receiverType)]; ok {
			platformMembers := semaTypeMembersFromPlatformSymbol(*platformSymbol)
			if platformMethods := platformMembers.methods[normalizeName(method)]; len(platformMethods) > 0 {
				candidates = make([]resolvedMember, 0, len(platformMethods))
				for _, platformMethod := range platformMethods {
					candidates = append(candidates, resolvedMember{owner: platformMembers.name, member: platformMethod})
				}
			}
		}
	}
	if len(candidates) == 0 {
		canonical := semaCanonicalPlatformAlias(receiverType)
		if !strings.EqualFold(canonical, receiverType) {
			candidates = resolveMemberMethods(model, canonical, method)
		}
	}
	if len(candidates) == 0 {
		return semaCollectionSignature{}, false
	}
	candidates = filterResolvedMethodsByReceiverMode(candidates, receiverMode)
	if len(candidates) == 0 {
		return semaCollectionSignature{}, false
	}
	if owner, ok := model.lookup(normalizeName(candidates[0].owner)); !ok || (!owner.dependency && !owner.sobject) {
		return semaCollectionSignature{}, false
	}
	returnType := strings.TrimSpace(candidates[0].member.Type)
	params := make([][]string, 0, len(candidates))
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		memberParams := make([]string, 0, len(candidate.member.Parameters))
		for _, param := range candidate.member.Parameters {
			memberParams = append(memberParams, param.Type)
		}
		signature := strings.Join(memberParams, "\x00")
		if seen[signature] {
			continue
		}
		seen[signature] = true
		memberReturn := strings.TrimSpace(candidate.member.Type)
		if memberReturn == "" {
			memberReturn = "void"
		}
		if returnType == "" {
			returnType = memberReturn
		}
		params = append(params, memberParams)
	}
	if returnType == "" {
		returnType = "void"
	}
	return semaCollectionSignature{returnType: returnType, params: params}, true
}
