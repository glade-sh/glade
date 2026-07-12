package sema

import (
	"reflect"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSemaStandardSObjectMembersCacheIsNamesOnly(t *testing.T) {
	resetSemaStandardSObjectMembersCacheForTest()
	defer resetSemaStandardSObjectMembersCacheForTest()

	names, members := semaStandardSObjectMembers()
	if len(names) == 0 {
		t.Fatalf("semaStandardSObjectMembers returned no names")
	}
	if len(members) != 0 {
		t.Fatalf("semaStandardSObjectMembers eagerly materialized %d member sets", len(members))
	}
	if name, ok := semaStandardSObjectNameForKey(normalizeName("Account")); !ok || name != "Account" {
		t.Fatalf("semaStandardSObjectNameForKey(Account) = %q, %v", name, ok)
	}
	if len(members) != 0 {
		t.Fatalf("semaStandardSObjectNameForKey materialized %d member sets", len(members))
	}
}

func resetSemaStandardSObjectMembersCacheForTest() {
	semaStandardSObjectMembersCache = struct {
		once      sync.Once
		names     []string
		members   map[string]typeMembers
		nameByKey map[string]string
	}{}
}

func TestAddStandardSObjectMembersUsesLazyPlaceholdersForUnreferencedObjects(t *testing.T) {
	resetSemaStandardSObjectMembersCacheForTest()
	defer resetSemaStandardSObjectMembersCacheForTest()

	model := buildTypeMembers(typesys.Index{})
	account, ok := model.members[normalizeName("Account")]
	if !ok {
		t.Fatalf("missing Account placeholder")
	}
	if account.fields != nil {
		t.Fatalf("Account placeholder eagerly materialized %d fields", len(account.fields))
	}
}

func TestTypeMemberCurrentOverlaySharesBase(t *testing.T) {
	first := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Duplicate",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "firstField", Type: "String"},
			{Kind: apexast.DeclarationMethod, Name: "firstHelper", Type: "void"},
		},
	}
	second := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Duplicate",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "secondField", Type: "Integer"},
			{Kind: apexast.DeclarationMethod, Name: "secondHelper", Type: "void"},
		},
	}
	unrelated := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Unrelated",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "sharedField", Type: "Boolean"},
		},
	}

	base := buildTypeMembers(typesys.Index{Types: []typesys.TypeSymbol{first, unrelated}})
	before := base.members[normalizeName(first.Name)]
	state := newSemaTypeMemberState(base)
	view := semaModelWithCurrentType(state.view(), second)

	if got := len(view.current); got > 2 {
		t.Fatalf("current overlay has %d aliases, want at most 2", got)
	}
	current, _, ok := semaLookupTypeMembers(view, second.Name)
	if !ok {
		t.Fatal("current duplicate type is missing from overlay")
	}
	if _, ok := current.fields[normalizeName("secondField")]; !ok {
		t.Fatalf("current duplicate fields = %#v, want secondField", current.fields)
	}
	if _, ok := current.methods[normalizeName("secondHelper")]; !ok {
		t.Fatalf("current duplicate methods = %#v, want secondHelper", current.methods)
	}
	shared, _, ok := semaLookupTypeMembers(view, unrelated.Name)
	if !ok || shared.name != unrelated.Name {
		t.Fatalf("unrelated lookup = %#v, %v; want base member", shared, ok)
	}
	if !reflect.DeepEqual(base.members[normalizeName(first.Name)], before) {
		t.Fatalf("base duplicate members changed\nwant: %#v\n got: %#v", before, base.members[normalizeName(first.Name)])
	}
	if _, ok := base.members[normalizeName(second.Name)].fields[normalizeName("secondField")]; ok {
		t.Fatalf("current duplicate field leaked into base: %#v", base.members[normalizeName(second.Name)].fields)
	}
}

func TestTypeMemberShortCandidateCollisionPreservesIndexOrder(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "First.Shared"},
		{Kind: apexast.DeclarationClass, Name: "Second.Shared"},
	}}
	model := buildTypeMembers(index)
	want := []string{normalizeName("First.Shared"), normalizeName("Second.Shared")}
	if got := model.shortCandidates[normalizeName("Shared")]; !reflect.DeepEqual(got, want) {
		t.Fatalf("Shared candidates = %#v, want %#v", got, want)
	}
}
