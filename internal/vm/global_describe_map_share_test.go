package vm

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// The global describe maps a VM builds (base, named-reference extension,
// change-event extension) belong to one runtime schema generation. VMs of one
// primed base share their backing until a VM writes; every outcome stays what
// it was when each VM owned private maps.

// globalDescribeMapDigest records what a map holds: key, token type and the
// token's object name. A shared map that any VM wrote changes its digest.
func globalDescribeMapDigest(entries map[string]Value) map[string]string {
	out := make(map[string]string, len(entries))
	for key, token := range entries {
		out[key] = token.Type + "/" + token.Fields["object"].Text
	}
	return out
}

func globalDescribeMapBacking(t *testing.T, machine *VM) Value {
	t.Helper()
	value := machine.globalDescribeMapValue()
	if len(value.Map) == 0 {
		t.Fatal("empty global describe map")
	}
	return value
}

// Fail-before: clones allocate no private copy of any layer.
func TestGlobalDescribeMapsSharedBetweenClones(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			first := describeShareClone(base, template)
			second := describeShareClone(base, template)
			if !sameMapBacking(first.schemaGlobalDescribe().Map, second.schemaGlobalDescribe().Map) {
				t.Fatal("clones hold private copies of the base global describe map")
			}
			if namespaced {
				// A namespaced result is a per-call projection; only the base is shared.
				return
			}
			left, right := globalDescribeMapBacking(t, first), globalDescribeMapBacking(t, second)
			if len(left.Map) <= len(first.schemaGlobalDescribe().Map) {
				t.Fatal("extension layers were not applied")
			}
			if !sameMapBacking(left.Map, right.Map) {
				t.Fatal("clones hold private copies of the extended global describe map")
			}
			again := globalDescribeMapBacking(t, first)
			if !sameMapBacking(again.Map, left.Map) {
				t.Fatal("a second call rebuilt the extended map")
			}
		})
	}
}

// Shared maps hold what a private build holds: same keys, same order, same
// tokens, for every layer and API version.
func TestGlobalDescribeMapsEqualPrivateBuild(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			want := globalDescribeMapBacking(t, describeShareUnprimedFor(namespaced))
			base, template := describeSharePrimedBaseFor(t, namespaced)
			for range 2 {
				got := globalDescribeMapBacking(t, describeShareClone(base, template))
				if len(got.Map) != len(want.Map) || len(got.MapKeys) != len(want.MapKeys) {
					t.Fatalf("size = %d/%d, want %d/%d", len(got.Map), len(got.MapKeys), len(want.Map), len(want.MapKeys))
				}
				for key, token := range want.Map {
					other, ok := got.Map[key]
					if !ok || other.Fields["object"].Text != token.Fields["object"].Text || other.Type != token.Type {
						t.Fatalf("token %q differs", key)
					}
				}
				if !reflect.DeepEqual(got.MapOrder, want.MapOrder) {
					t.Fatal("key order differs")
				}
			}
			source := `
Map<String,Schema.SObjectType> m = Schema.getGlobalDescribe();
System.assert(m.containsKey('account'));
System.assert(m.containsKey('accountchangeevent'));
System.assertEquals(m.size(), m.keySet().size());
System.assertEquals(m.size(), m.values().size());
System.assertEquals(Account.SObjectType, m.get('ACCOUNT'));`
			describeShareRunApex(t, describeShareUnprimedFor(namespaced), source)
			describeShareRunApex(t, describeShareClone(base, template), source)
			describeShareRunApex(t, describeShareClone(base, template), source)
		})
	}
}

// Every write route in one VM leaves the shared maps and every other VM as
// they were, and the writer reads its own write.
func TestGlobalDescribeSharedMapsSurviveWrites(t *testing.T) {
	routes := []struct{ name, source string }{
		{"put", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.put('private', Account.SObjectType); System.assert(h.containsKey('private'));`},
		{"put over existing", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.put('account', User.SObjectType); System.assertEquals(User.SObjectType, h.get('account'));`},
		{"remove", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); Integer before = h.size(); h.remove('account'); System.assert(h.size() < before);`},
		{"clear", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.clear(); System.assertEquals(0, h.size());`},
		{"putAll", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.putAll(new Map<String,Schema.SObjectType>{'private' => User.SObjectType}); System.assert(h.containsKey('private'));`},
		{"keySet remove", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.keySet().remove('account');`},
		{"keySet clear", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.keySet().clear();`},
		{"keySet retainAll", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.keySet().retainAll(new Set<String>{'user'});`},
		{"values clear", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.values().clear();`},
		{"unchained put", `Schema.getGlobalDescribe().put('private', Account.SObjectType);`},
		{"unchained clear", `Schema.getGlobalDescribe().clear();`},
		{"app describe retyped", `Map<String,Schema.SObjectType> h = Schema.getAppDescribe('x'); h.put('private', Account.SObjectType);`},
		{"module describe retyped", `Map<String,Schema.SObjectType> h = Schema.getModuleDescribe(); h.clear();`},
		{"differently spelled type", `Map<String,SObjectType> h = Schema.getGlobalDescribe(); h.put('private', Account.SObjectType); h.remove('user');`},
		{"cloned map", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe().clone(); h.clear();`},
		{"token write", `Schema.getGlobalDescribe().get('account').object = 'Contact';`},
		{"change event token write", `Schema.getGlobalDescribe().get('accountchangeevent').object = 'Contact';`},
		{"token write then put", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.get('account').object = 'Contact'; h.put('private', User.SObjectType); System.assertEquals('Contact', String.valueOf(h.get('account')));`},
		{"put then token write", `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.put('private', User.SObjectType); h.get('account').object = 'Contact'; System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));`},
	}
	for _, namespaced := range []bool{false, true} {
		for _, route := range routes {
			t.Run(fmt.Sprintf("namespace=%v/%s", namespaced, route.name), func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, namespaced)
				early := describeShareClone(base, template)
				pristineBase := globalDescribeMapDigest(early.schemaGlobalDescribe().Map)
				pristineExtended := globalDescribeMapDigest(globalDescribeMapBacking(t, early).Map)
				writer := describeShareClone(base, template)
				describeShareRunApex(t, writer, route.source)
				// Writing twice, in a second cache generation of the same VM, must not matter.
				describeShareRunApex(t, writer, route.source)
				late := describeShareClone(base, template)
				for name, machine := range map[string]*VM{"early": early, "late": late} {
					if got := globalDescribeMapDigest(machine.schemaGlobalDescribe().Map); !reflect.DeepEqual(got, pristineBase) {
						t.Fatalf("%s VM sees a changed base map", name)
					}
					if got := globalDescribeMapDigest(globalDescribeMapBacking(t, machine).Map); !reflect.DeepEqual(got, pristineExtended) {
						t.Fatalf("%s VM sees a changed extended map", name)
					}
					describeShareRunApex(t, machine, `
Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe();
System.assert(h.containsKey('account'));
System.assert(h.containsKey('user'));
System.assert(!h.containsKey('private'));
System.assertEquals('Account', String.valueOf(h.get('account')));
System.assertEquals(Account.SObjectType, h.get('account'));
System.assertEquals(h.size(), h.keySet().size());`)
				}
			})
		}
	}
}

// A static field that holds a describe map or token before its VM is cloned
// keeps its contents when the clone writes through the same static.
func TestGlobalDescribeStaticHoldersSurviveCloneWrites(t *testing.T) {
	mutations := []string{
		`DescribeOwner.stored.put('private', Account.SObjectType);`,
		`DescribeOwner.stored.remove('account');`,
		`DescribeOwner.stored.clear();`,
		`DescribeOwner.stored.get('account').object = 'Contact';`,
		`DescribeOwner.token.object = 'Contact';`,
	}
	for _, namespaced := range []bool{false, true} {
		for _, mutation := range mutations {
			t.Run(fmt.Sprintf("namespace=%v/%s", namespaced, mutation), func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, namespaced)
				origin := describeShareClone(base, template)
				if err := origin.RegisterClass(Class{Name: "DescribeOwner",
					StaticFields: map[string]Field{
						"stored": {Name: "stored", Type: "Map<String,Schema.SObjectType>", Static: true},
						"token":  {Name: "token", Type: "Schema.SObjectType", Static: true},
					}}); err != nil {
					t.Fatal(err)
				}
				seed, err := CompileAnonymous(`DescribeOwner.stored = Schema.getGlobalDescribe(); DescribeOwner.token = Schema.getGlobalDescribe().get('account');`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := origin.ExecuteInClass(seed, "DescribeOwner"); err != nil {
					t.Fatal(err)
				}
				pristine := globalDescribeMapDigest(origin.schemaGlobalDescribe().Map)
				clone := origin.CloneRuntime(nil)
				org := template.CloneRuntimeOrg()
				clone.SetRuntimeTemplateOrg(&org)
				clone.EnableTestContext()
				write, err := CompileAnonymous(mutation)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := clone.ExecuteInClass(write, "DescribeOwner"); err != nil {
					t.Fatal(err)
				}
				check, err := CompileAnonymous(`
System.assertEquals('Account', String.valueOf(DescribeOwner.token));
System.assert(DescribeOwner.stored.containsKey('account'));
System.assert(!DescribeOwner.stored.containsKey('private'));
System.assertEquals('Account', String.valueOf(DescribeOwner.stored.get('account')));
System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := origin.ExecuteInClass(check, "DescribeOwner"); err != nil {
					t.Fatalf("origin VM changed by its clone: %v", err)
				}
				if got := globalDescribeMapDigest(origin.schemaGlobalDescribe().Map); !reflect.DeepEqual(got, pristine) {
					t.Fatal("origin base map changed by its clone")
				}
				later := describeShareClone(base, template)
				if got := globalDescribeMapDigest(later.schemaGlobalDescribe().Map); !reflect.DeepEqual(got, pristine) {
					t.Fatal("shared base map changed by a clone's static write")
				}
			})
		}
	}
}

// A write that clones the VM's map keeps the VM's later describe calls on its
// own copy, and a VM that never wrote keeps the shared one.
func TestGlobalDescribeWriterDetachesOnlyItself(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	reader := describeShareClone(base, template)
	writer := describeShareClone(base, template)
	shared := reader.schemaGlobalDescribe().Map
	describeShareRunApex(t, writer, `Schema.getGlobalDescribe().put('private', Account.SObjectType);`)
	if sameMapBacking(writer.schemaGlobalDescribe().Map, shared) {
		t.Fatal("writer still holds the shared map")
	}
	if !sameMapBacking(reader.schemaGlobalDescribe().Map, shared) {
		t.Fatal("reader lost the shared map")
	}
	describeShareRunApex(t, writer, `System.assert(Schema.getGlobalDescribe().containsKey('private'));`)
	describeShareRunApex(t, reader, `System.assert(!Schema.getGlobalDescribe().containsKey('private'));`)
	if !sameMapBacking(describeShareClone(base, template).schemaGlobalDescribe().Map, shared) {
		t.Fatal("a new clone did not receive the shared map")
	}
}

// Many VMs of one base build, read and write the shared maps at once. Run with
// -race: any in-place write to a shared map is a data race.
func TestGlobalDescribeMapsConcurrentClones(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			var group sync.WaitGroup
			errs := make(chan error, 32)
			start := make(chan struct{})
			for worker := 0; worker < 24; worker++ {
				group.Add(1)
				go func(worker int) {
					defer group.Done()
					machine := describeShareClone(base, template)
					var source string
					switch worker % 4 {
					case 0:
						source = `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); System.assert(h.containsKey('account')); System.assertEquals(h.size(), h.keySet().size());`
					case 1:
						source = `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.put('private', Account.SObjectType); Integer before = h.size(); h.remove('user'); System.assert(h.size() < before);`
					case 2:
						source = `Schema.getGlobalDescribe().get('account').object = 'Worker'; System.assertEquals('Worker', String.valueOf(Schema.getGlobalDescribe().get('account')));`
					default:
						source = `Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe(); h.clear(); System.assertEquals(0, h.size());`
					}
					program, err := CompileAnonymous(source)
					if err != nil {
						errs <- err
						return
					}
					<-start
					if _, err := machine.Execute(program); err != nil {
						errs <- err
					}
				}(worker)
			}
			close(start)
			group.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			describeShareRunApex(t, describeShareClone(base, template), `
Map<String,Schema.SObjectType> h = Schema.getGlobalDescribe();
System.assert(h.containsKey('user'));
System.assert(!h.containsKey('private'));
System.assertEquals('Account', String.valueOf(h.get('account')));`)
		})
	}
}

func globalDescribeMapAtAPI(t *testing.T, machine *VM, api string) map[string]string {
	t.Helper()
	machine.currentMethod.APIVersion = api
	return globalDescribeMapDigest(globalDescribeMapBacking(t, machine).Map)
}

// The change-event layer depends on the API version. One VM switching versions,
// and a write at either version, leave the shared layer of each version, and
// every other VM, as a private build reads it.
func TestGlobalDescribeMapsAcrossAPIVersions(t *testing.T) {
	versions := []string{"62.0", "67.0"}
	writes := map[string]string{
		"put":         `Schema.getGlobalDescribe().put('private', Account.SObjectType);`,
		"clear":       `Schema.getGlobalDescribe().clear();`,
		"token write": `Schema.getGlobalDescribe().get('accountchangeevent').object = 'Contact';`,
	}
	for _, namespaced := range []bool{false, true} {
		want := map[string]map[string]string{}
		for _, api := range versions {
			want[api] = globalDescribeMapAtAPI(t, describeShareUnprimedFor(namespaced), api)
		}
		for name, source := range writes {
			for _, writeAPI := range versions {
				t.Run(fmt.Sprintf("namespace=%v/%s/write at %s", namespaced, name, writeAPI), func(t *testing.T) {
					base, template := describeSharePrimedBaseFor(t, namespaced)
					early := describeShareClone(base, template)
					for _, api := range versions {
						if got := globalDescribeMapAtAPI(t, early, api); !reflect.DeepEqual(got, want[api]) {
							t.Fatalf("shared layer at %s differs from a private build", api)
						}
					}
					writer := describeShareClone(base, template)
					for _, api := range versions {
						globalDescribeMapAtAPI(t, writer, api)
					}
					writer.currentMethod.APIVersion = writeAPI
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: writeAPI})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := writer.Execute(program); err != nil {
						t.Fatal(err)
					}
					// Back to the other version on the writer, then read from fresh VMs.
					for _, machine := range []*VM{early, describeShareClone(base, template)} {
						for _, api := range versions {
							if got := globalDescribeMapAtAPI(t, machine, api); !reflect.DeepEqual(got, want[api]) {
								t.Fatalf("write at %s changed the layer another VM reads at %s", writeAPI, api)
							}
						}
					}
				})
			}
		}
	}
}
