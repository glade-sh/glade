package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// strictHeapShapesOrg builds a schema-backed org so standard objects resolve.
func strictHeapShapesOrg(tb testing.TB) (*storage.OrgState, typesys.Index) {
	tb.Helper()
	root := tb.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`), 0644); err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "force-app"), 0755); err != nil {
		tb.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		tb.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		tb.Fatal(err)
	}
	index := typesys.Build(p, s)
	org := apextest.OrgFromIndex(index)
	org.APIVersion = "67.0"
	return &org, index
}

// strictHeapShapeSources are realistic list and map building loops. Each
// takes the row count n.
var strictHeapShapeSources = map[string]func(n int) string{
	"nested-new-3": func(n int) string {
		return fmt.Sprintf(`List<User> users = new List<User>();
for (Integer i = 0; i < 50; i++) { users.add(new User(LastName = 'u' + i, Username = 'u' + i + '@x.test', Alias = 'a' + i, Email = 'e@x.test', TimeZoneSidKey = 'GMT', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US')); }
insert users;
Id psId = '0PS000000000001AAA'; Id c1 = '003000000000000AAA';
List<PermissionSetAssignment> rows = new List<PermissionSetAssignment>();
for (User u : users) {
  for (Integer j = 0; j < %d; j++) {
    rows.add(new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = psId, SystemModstamp = null));
  }
}`, n/50)
	},
	"nested-new-2": func(n int) string {
		return fmt.Sprintf(`List<Account> users = new List<Account>();
for (Integer i = 0; i < 50; i++) { users.add(new Account(Name = 'u' + i)); }
insert users;
List<Contact> rows = new List<Contact>();
for (Account a : users) {
  for (Integer j = 0; j < %d; j++) {
    rows.add(new Contact(LastName = 'c' + j, AccountId = a.Id));
  }
}`, n/50)
	},
	"nested-new-4-lit": func(n int) string {
		return fmt.Sprintf(`List<Contact> rows = new List<Contact>();
for (Integer i = 0; i < 50; i++) {
  for (Integer j = 0; j < %d; j++) {
    rows.add(new Contact(LastName = 'c' + j, FirstName = 'f' + i, Email = 'a@b.c', Title = 't'));
  }
}`, n/50)
	},
	"foreach-flat": func(n int) string {
		return fmt.Sprintf(`List<Account> accs = new List<Account>();
for (Integer i = 0; i < %d; i++) { accs.add(new Account(Name = 'u' + i)); }
insert accs;
List<Contact> rows = new List<Contact>();
for (Account a : accs) { rows.add(new Contact(LastName = a.Name, AccountId = a.Id)); }`, n)
	},
	"foreach-map-put": func(n int) string {
		return fmt.Sprintf(`List<Account> accs = new List<Account>();
for (Integer i = 0; i < %d; i++) { accs.add(new Account(Name = 'u' + i)); }
insert accs;
Map<Id, Account> byId = new Map<Id, Account>();
for (Account a : accs) { byId.put(a.Id, a); }`, n)
	},
	"foreach-ids": func(n int) string {
		return fmt.Sprintf(`List<Account> accs = new List<Account>();
for (Integer i = 0; i < %d; i++) { accs.add(new Account(Name = 'u' + i)); }
insert accs;
List<Id> ids = new List<Id>();
for (Account a : accs) { ids.add(a.Id); }`, n)
	},
	"add-existing": func(n int) string {
		return fmt.Sprintf(`List<Contact> rows = new List<Contact>();
Contact c = new Contact(LastName = 'x');
for (Integer i = 0; i < %d; i++) { rows.add(c); }`, n)
	},
	"add-var-built": func(n int) string {
		return fmt.Sprintf(`List<Contact> rows = new List<Contact>();
for (Integer i = 0; i < %d; i++) { Contact c = new Contact(LastName = 'x' + i); c.Email = 'e@x.y'; rows.add(c); }`, n)
	},
	"map-put-new": func(n int) string {
		return fmt.Sprintf(`Map<String, Contact> m = new Map<String, Contact>();
for (Integer i = 0; i < %d; i++) { m.put('k' + i, new Contact(LastName = 'x' + i)); }`, n)
	},
}

func BenchmarkStrictHeapShapes(b *testing.B) {
	for name, gen := range strictHeapShapeSources {
		for _, n := range []int{1000, 5000} {
			source := gen(n)
			b.Run(fmt.Sprintf("%s/%d", name, n), func(b *testing.B) {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "67.0"})
				if err != nil {
					b.Fatal(err)
				}
				org, index := strictHeapShapesOrg(b)
				priced, walked := 0, 0
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					m := vm.New(nil)
					m.SetOrg(org)
					if err := apextest.RegisterProjectRuntime(m, index); err != nil {
						b.Fatal(err)
					}
					m.SetLimitMode(vm.LimitModeStrict)
					b.StartTimer()
					if _, err := m.Execute(program); err != nil {
						b.Fatal(err)
					}
					priced, walked = m.HeapChecksPricedForTest(), m.HeapChecksWalkedForTest()
				}
				b.ReportMetric(float64(priced), "priced")
				b.ReportMetric(float64(walked), "walked")
			})
		}
	}
}

var benchSizes = []int{1000, 5000}
