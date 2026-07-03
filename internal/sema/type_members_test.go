package sema

import (
	"sync"
	"testing"

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
	account, ok := model[normalizeName("Account")]
	if !ok {
		t.Fatalf("missing Account placeholder")
	}
	if account.fields != nil {
		t.Fatalf("Account placeholder eagerly materialized %d fields", len(account.fields))
	}
}
