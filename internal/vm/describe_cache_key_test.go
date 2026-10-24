package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestObjectNameLookupNativeConformance(t *testing.T) {
	const dir = "testdata/object_name_lookup/"
	source, err := os.ReadFile(dir + "probe.apex")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,row,observation" {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want []string
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 3 {
					t.Fatalf("invalid row: %v", row)
				}
				if row[0] == api {
					if seen[row[1]] {
						t.Fatalf("duplicate row: %v", row)
					}
					seen[row[1]] = true
					want = append(want, "ROW\t"+row[1]+"\t"+row[2])
				}
			}
			if len(want) != 4 {
				t.Fatal("missing native observations")
			}
			org := describeShareOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: api, SourceContextBound: true}
			program, err := CompileAnonymous(string(source))
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(result.Debug, "\n"); got != strings.Join(want, "\n") {
				t.Fatalf("Glade:\n%s\nnative:\n%s", got, strings.Join(want, "\n"))
			}
		})
	}
}

func describeKeyMachine() *VM {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005000000000001aaa")
	machine.executionUser.Fields["ProfileId"] = String("00e000000000001aaa")
	return machine
}

// Lowercase identifiers isolate cache-key construction from name folding,
// which has its own implementation and allocation tests.
func TestDescribeCacheKeyWarmAllocations(t *testing.T) {
	for _, withUser := range []bool{false, true} {
		machine := describeKeyMachine()
		maxAllocs := float64(2)
		context := "with-user/"
		if !withUser {
			machine.executionUser = Null
			maxAllocs = 0
			context = "without-user/"
		}
		t.Run(context+"object", func(t *testing.T) {
			want := machine.describeSObjectCacheKey("account")
			allocs := testing.AllocsPerRun(100, func() {
				if got := machine.describeSObjectCacheKey("account"); got != want {
					t.Fatal("unstable object key")
				}
			})
			t.Logf("warm object key allocations = %.0f", allocs)
			if allocs > maxAllocs {
				t.Fatalf("warm object key allocated %.0f times; want <=%.0f", allocs, maxAllocs)
			}
		})
		t.Run(context+"field", func(t *testing.T) {
			want := machine.fieldDescribeCacheKey("account", "name")
			allocs := testing.AllocsPerRun(100, func() {
				if got := machine.fieldDescribeCacheKey("account", "name"); got != want {
					t.Fatal("unstable field key")
				}
			})
			t.Logf("warm field key allocations = %.0f", allocs)
			if allocs > maxAllocs {
				t.Fatalf("warm field key allocated %.0f times; want <=%.0f", allocs, maxAllocs)
			}
		})
	}
}

func TestDescribeCacheKeyScopeControls(t *testing.T) {
	machine := describeKeyMachine()
	object := machine.describeSObjectCacheKey("Account")
	field := machine.fieldDescribeCacheKey("Account", "Name")
	if object != machine.describeSObjectCacheKey(" account ") || field != machine.fieldDescribeCacheKey(" ACCOUNT ", " name ") {
		t.Fatal("case or whitespace changed describe identity")
	}
	if object == machine.describeSObjectCacheKey("Contact") || field == machine.fieldDescribeCacheKey("Account", "Id") || field == machine.fieldDescribeCacheKey("Contact", "Name") {
		t.Fatal("distinct describe targets share a key")
	}
	machine.executionUser.Fields["ProfileId"] = String("00e000000000002aaa")
	if object == machine.describeSObjectCacheKey("Account") || field == machine.fieldDescribeCacheKey("Account", "Name") {
		t.Fatal("profile change reused key")
	}
	machine.executionUser.Fields["ProfileId"] = String("00e000000000001aaa")
	machine.executionUser.Fields["Id"] = String("005000000000002aaa")
	if object == machine.describeSObjectCacheKey("Account") {
		t.Fatal("user change reused key")
	}
	machine.EnableTestContext()
	machine.testContext.CurrentUser = Object("User")
	machine.testContext.CurrentUser.Fields["Id"] = String("005000000000001aaa")
	machine.testContext.CurrentUser.Fields["ProfileId"] = String("00e000000000001aaa")
	if object != machine.describeSObjectCacheKey("Account") {
		t.Fatal("test user did not override execution user")
	}
	machine.testContext.CurrentUser = Value{}
	if object == machine.describeSObjectCacheKey("Account") {
		t.Fatal("execution user did not resume")
	}
}

func TestDescribeCacheKeyAssignmentMutation(t *testing.T) {
	machine := describeKeyMachine()
	before := machine.describeSObjectCacheKey("Account")
	permissionIdentityPut(machine, "PermissionSet", "0ps000000000001", nil)
	permissionIdentityPut(machine, "PermissionSet", "0ps000000000002", nil)
	permissionIdentityPut(machine, "PermissionSetAssignment", "0pa000000000001", map[string]storage.Value{
		"AssigneeId":      storage.IDValue("005000000000001aaa"),
		"PermissionSetId": storage.IDValue("0ps000000000001"),
	})
	assigned := machine.describeSObjectCacheKey("Account")
	if before == assigned {
		t.Fatal("assignment insertion reused key")
	}
	row := machine.Org.Objects["PermissionSetAssignment"].Records["0pa000000000001"]
	row.Fields["PermissionSetId"] = storage.IDValue("0ps000000000002")
	if got := machine.describeSObjectCacheKey("Account"); got == assigned || got == before {
		t.Fatal("same-count assignment update reused key")
	}
	delete(machine.Org.Objects["PermissionSetAssignment"].Records, "0pa000000000001")
	if got := machine.describeSObjectCacheKey("Account"); got != before {
		t.Fatal("assignment deletion did not restore key")
	}
}

func BenchmarkDescribeCacheKey(b *testing.B) {
	machine := describeKeyMachine()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine.fieldDescribeCacheKey("account", "name")
	}
}

func TestDescribeCacheKeyWarmFieldLookupAllocations(t *testing.T) {
	machine := describeKeyMachine()
	machine.fieldDescribeCache[machine.fieldDescribeCacheKey("account", "name")] = Int(7)
	allocs := testing.AllocsPerRun(100, func() {
		value, ok := machine.lookupFieldDescribeCache("account", "name")
		if !ok || value.Kind != ValueInt || value.Int != 7 {
			t.Fatal("cached field describe changed")
		}
	})
	t.Logf("warm field cache lookup allocations = %.0f", allocs)
	if allocs > 2 {
		t.Fatalf("warm field cache lookup allocated %.0f times; want <=2", allocs)
	}
}

func BenchmarkDescribeFieldCacheLookup(b *testing.B) {
	machine := describeKeyMachine()
	machine.fieldDescribeCache[machine.fieldDescribeCacheKey("account", "name")] = Int(7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine.lookupFieldDescribeCache("account", "name")
	}
}
