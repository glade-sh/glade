package vm

import (
	"fmt"
	"sync"
	"testing"
)

// Describe tokens are immutable data shared by clones of a primed base. A VM
// privatizes a token on its first write; every outcome stays what it was when
// each VM owned a private copy. The Apex bodies below hold for the private-copy
// baseline and for the shared tokens.

func tokenFieldsBacking(t *testing.T, machine *VM, key string) Value {
	t.Helper()
	value := machine.schemaGlobalDescribe()
	token, ok := value.Map[mapKey(String(key))]
	if !ok {
		t.Fatalf("no %q token", key)
	}
	return token
}

// Fail-before: clones share one token set until a write.
func TestGlobalDescribeTokensSharedBetweenClones(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			first := describeShareClone(base, template)
			second := describeShareClone(base, template)
			left := tokenFieldsBacking(t, first, "account")
			right := tokenFieldsBacking(t, second, "account")
			if !sameMapBacking(left.Fields, right.Fields) {
				t.Fatal("clones privatized token fields before any write")
			}
			if left.Ref != right.Ref {
				t.Fatal("clones assigned distinct token identities before any write")
			}
		})
	}
}

func TestGlobalDescribeTokenEquality(t *testing.T) {
	source := `
Map<String,Schema.SObjectType> first = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> second = Schema.getGlobalDescribe();
System.assert(first.get('account') == second.get('account'));
System.assert(first.get('account') == Account.SObjectType);
System.assert(Account.SObjectType == first.get('account'));
System.assert(first.get('account') != first.get('user'));
System.assertEquals(User.SObjectType, second.get('user'));
System.assertEquals('Account', String.valueOf(first.get('account')));
Set<Schema.SObjectType> seen = new Set<Schema.SObjectType>(first.values());
System.assert(seen.contains(Account.SObjectType));
System.assert(seen.contains(User.SObjectType));
Map<Schema.SObjectType,String> byToken = new Map<Schema.SObjectType,String>();
byToken.put(first.get('account'), 'a');
System.assertEquals('a', byToken.get(Account.SObjectType));
`
	for _, namespaced := range []bool{false, true} {
		base, template := describeSharePrimedBaseFor(t, namespaced)
		describeShareRunApex(t, describeShareClone(base, template), source)
		describeShareRunApex(t, describeShareClone(base, template), source)
		describeShareRunApex(t, describeShareUnprimedFor(namespaced), source)
	}
}

// Writes to a token reach every holder in the writing VM and no other VM,
// test, or later read of the shared set.
func TestGlobalDescribeTokenMutationIsolation(t *testing.T) {
	routes := []struct{ name, source string }{
		{"variable alias", `
Schema.SObjectType a = Schema.getGlobalDescribe().get('account');
Schema.SObjectType b = Schema.getGlobalDescribe().get('account');
a.object = 'Contact';
System.assertEquals('Contact', String.valueOf(a));
System.assertEquals('Contact', String.valueOf(b));
System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));
`},
		{"typed holder map", `
Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
holder.get('account').object = 'Contact';
System.assertEquals('Contact', String.valueOf(holder.get('account')));
System.assertEquals('Contact', String.valueOf(sibling.get('account')));
System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));
`},
		{"detached holder after put", `
Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
holder.put('private', Account.SObjectType);
Schema.SObjectType token = holder.get('account');
token.object = 'Contact';
System.assertEquals('Contact', String.valueOf(holder.get('account')));
System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));
System.assertEquals('Account', String.valueOf(holder.get('private')));
`},
		{"values loop", `
for (Schema.SObjectType token : Schema.getGlobalDescribe().values()) { token.object = 'Contact'; }
System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));
`},
		{"values list", `
List<Schema.SObjectType> tokens = Schema.getGlobalDescribe().values();
for (Schema.SObjectType token : tokens) { token.object = 'Contact'; }
for (Schema.SObjectType token : tokens) { System.assertEquals('Contact', String.valueOf(token)); }
`},
		{"cloned map", `
Map<String,Schema.SObjectType> cloned = Schema.getGlobalDescribe().clone();
Schema.SObjectType token = cloned.get('account');
token.object = 'Contact';
System.assertEquals('Contact', String.valueOf(cloned.get('account')));
System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));
`},
		{"compound assign", `
Schema.SObjectType token = Schema.getGlobalDescribe().get('account');
token.object += 'X';
System.assertEquals('AccountX', String.valueOf(Schema.getGlobalDescribe().get('account')));
`},
	}
	for _, namespaced := range []bool{false, true} {
		for _, route := range routes {
			t.Run(fmt.Sprintf("namespace=%v/%s", namespaced, route.name), func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, namespaced)
				before := describeShareClone(base, template)
				writer := describeShareClone(base, template)
				pristine := tokenFieldsBacking(t, before, "account").Fields["object"].Text
				describeShareRunApex(t, writer, route.source)
				// Another VM, a later VM, and the shared set stay untouched.
				after := describeShareClone(base, template)
				for _, machine := range []*VM{before, after} {
					describeShareRunApex(t, machine, `
System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));
System.assertEquals('User', String.valueOf(Schema.getGlobalDescribe().get('user')));
for (Schema.SObjectType token : Schema.getGlobalDescribe().values()) {
	System.assertNotEquals('AccountX', String.valueOf(token));
}`)
				}
				if got := tokenFieldsBacking(t, after, "account").Fields["object"].Text; got != pristine {
					t.Fatalf("shared token changed: %q", got)
				}
				if got := tokenFieldsBacking(t, before, "account").Fields["object"].Text; got != pristine {
					t.Fatalf("earlier VM token changed: %q", got)
				}
			})
		}
	}
}

// A write privatizes the token for this VM; other tokens stay shared, and a
// second write to the same token does not copy again.
func TestGlobalDescribeTokenWritePrivatizesOnce(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	reference := describeShareClone(base, template)
	writer := describeShareClone(base, template)
	describeShareRunApex(t, writer, `Schema.getGlobalDescribe().get('account').object = 'Contact';`)
	account := tokenFieldsBacking(t, writer, "account")
	if sameMapBacking(account.Fields, tokenFieldsBacking(t, reference, "account").Fields) {
		t.Fatal("written token still shares fields")
	}
	if !sameMapBacking(tokenFieldsBacking(t, writer, "user").Fields, tokenFieldsBacking(t, reference, "user").Fields) {
		t.Fatal("unwritten token was privatized")
	}
	describeShareRunApex(t, writer, `Schema.getGlobalDescribe().get('account').object = 'Lead';`)
	if !sameMapBacking(tokenFieldsBacking(t, writer, "account").Fields, account.Fields) {
		t.Fatal("second write copied the private token again")
	}
}

// put/remove on the map never reach the shared token set or another VM.
func TestGlobalDescribeMapPutRemoveIsolationWithSharedTokens(t *testing.T) {
	mutations := []string{
		`holder.put('private', Account.SObjectType);`,
		`holder.put('private', holder.get('account'));`,
		`holder.remove('account');`,
		`holder.putAll(new Map<String,Schema.SObjectType>{'private' => User.SObjectType});`,
		`holder.clear();`,
	}
	for _, namespaced := range []bool{false, true} {
		for _, mutation := range mutations {
			t.Run(fmt.Sprintf("namespace=%v/%s", namespaced, mutation), func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, namespaced)
				writer := describeShareClone(base, template)
				reader := describeShareClone(base, template)
				readerSize := reader.schemaGlobalDescribe()
				size := len(readerSize.Map)
				describeShareRunApex(t, writer, `Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
`+mutation+`
holder.put('another', User.SObjectType);
System.assert(holder.containsKey('another'));`)
				again := describeShareClone(base, template)
				for _, machine := range []*VM{reader, again} {
					describeShareRunApex(t, machine, `Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
System.assert(holder.containsKey('account'));
System.assert(!holder.containsKey('private'));
System.assert(!holder.containsKey('another'));
System.assertEquals(Account.SObjectType, holder.get('account'));`)
				}
				if got := len(again.schemaGlobalDescribe().Map); got != size {
					t.Fatalf("shared set size changed: %d != %d", got, size)
				}
			})
		}
	}
}

// Many VMs of one base read, write and mutate tokens at once. Run with -race.
func TestGlobalDescribeTokensConcurrentClones(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	// Build the shared set once, as the first test worker would.
	describeShareRunApex(t, describeShareClone(base, template), `Schema.getGlobalDescribe();`)
	var group sync.WaitGroup
	errs := make(chan error, 16)
	for worker := 0; worker < 16; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			machine := describeShareClone(base, template)
			name := fmt.Sprintf("Worker%d", worker)
			source := `Schema.SObjectType token = Schema.getGlobalDescribe().get('account');
System.assertEquals('Account', String.valueOf(token));
Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
holder.put('private', token);
if (` + fmt.Sprint(worker%2) + ` == 1) {
	token.object = '` + name + `';
	System.assertEquals('` + name + `', String.valueOf(Schema.getGlobalDescribe().get('account')));
} else {
	System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));
}`
			program, err := CompileAnonymous(source)
			if err != nil {
				errs <- err
				return
			}
			if _, err := machine.Execute(program); err != nil {
				errs <- err
			}
		}(worker)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	describeShareRunApex(t, describeShareClone(base, template), `System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));`)
}
