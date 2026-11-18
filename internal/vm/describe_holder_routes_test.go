package vm

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Baseline local mutation controls; native read/alias rows remain
// testdata/describe_read_workload/org.tsv:7-10,16-19. The shared fixture is also
// run by actual project and host routes; it never infers native writable maps.
// Key-set remove and clear write through to the holder and its aliases, never
// to a sibling or a fresh getGlobalDescribe() (native review_round10
// g_gd_remove, g_gd_clear; review_round11 gk_removeall, gk_retainall, gk_saved,
// gk_alias, gk_class). put, putAll and remove are read-only on every holder
// (review_round11 gw_put, gw_putall, gk_direct): see
// TestDescribeHolderDirectWritesAreReadOnly.
func TestDescribeHolderPublicationRoutes(t *testing.T) {
	raw, err := os.ReadFile("testdata/describe_ownership/holder.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			for _, caller := range []string{"", "zzns", "foreign"} {
				for _, primed := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/org=%v/caller=%s/primed=%v", api, namespaced, caller, primed), func(t *testing.T) {
						makeVM := func() *VM {
							if primed {
								base, template := describeSharePrimedBaseFor(t, namespaced)
								return describeShareClone(base, template)
							}
							return describeShareUnprimedFor(namespaced)
						}
						machine := makeVM()
						for _, write := range []struct{ mutation, size string }{
							{"clear()", "0"},
							{"keySet().remove('account')", "beforeSize - 1"},
							{"keySet().clear()", "0"},
							{"keySet().removeAll(new Set<String>{'account'})", "beforeSize - 1"},
							{"keySet().retainAll(new Set<String>{'user'})", "1"},
						} {
							mutation, err := CompileAnonymousWithOptions("m."+write.mutation+";", CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if err := machine.RegisterClass(Class{Name: "DescribeOwner", Namespace: caller,
								Fields:       map[string]Field{"member": {Name: "member", Type: "Map<String,Schema.SObjectType>"}},
								StaticFields: map[string]Field{"stored": {Name: "stored", Type: "Map<String,Schema.SObjectType>", Static: true}},
								Methods:      map[string]Method{"mutate": {Name: "DescribeOwner.mutate", ClassName: "DescribeOwner", IsStatic: true, ReturnType: "void", Params: []Param{{Name: "m", Type: "Map<String,Schema.SObjectType>"}}, Program: mutation}},
							}); err != nil {
								t.Fatal(err)
							}
							for _, target := range []string{"holder", "alias", "DescribeOwner.stored", "instance.member", "parameter"} {
								action := target + "." + write.mutation + ";"
								if target == "parameter" {
									action = "DescribeOwner.mutate(holder);"
								}
								source := strings.ReplaceAll(strings.ReplaceAll(string(raw), "@@WRITE@@", action), "@@SIZE@@", write.size)
								program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								// Repeated calls on the same VM cover both cold and warm caches.
								for range 2 {
									if _, err := machine.ExecuteInClass(program, "DescribeOwner"); err != nil {
										t.Fatalf("%s/%s: %v", target, write.mutation, err)
									}
								}
							}
						}
					})
				}
			}
		}
	}
}

// NATIVE review_round11 gw_put, gw_put_existing, gw_putall, gk_direct and
// gw_remove_missing (62.0, 67.0): put, putAll and remove on the global
// describe raise the uncatchable FinalException. gk_alias shows the local,
// static, field and parameter aliases name the same map.
func TestDescribeHolderDirectWritesAreReadOnly(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		for _, mutation := range []string{
			"put('private', Account.SObjectType)", "put('account', User.SObjectType)",
			"putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType})",
			"remove('account')", "remove('zzz_missing')",
		} {
			for _, target := range []string{"holder", "alias", "DescribeOwner.stored", "instance.member", "parameter"} {
				machine := describeShareUnprimedFor(namespaced)
				program, err := CompileAnonymousWithOptions("m."+mutation+";", CompileOptions{APIVersion: "67.0"})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "DescribeOwner",
					Fields:       map[string]Field{"member": {Name: "member", Type: "Map<String,Schema.SObjectType>"}},
					StaticFields: map[string]Field{"stored": {Name: "stored", Type: "Map<String,Schema.SObjectType>", Static: true}},
					Methods:      map[string]Method{"mutate": {Name: "DescribeOwner.mutate", ClassName: "DescribeOwner", IsStatic: true, ReturnType: "void", Params: []Param{{Name: "m", Type: "Map<String,Schema.SObjectType>"}}, Program: program}},
				}); err != nil {
					t.Fatal(err)
				}
				action := target + "." + mutation + ";"
				if target == "parameter" {
					action = "DescribeOwner.mutate(holder);"
				}
				source := `Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
try { ` + action + ` } catch (Exception e) { System.debug('caught'); }`
				compiled, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.ExecuteInClass(compiled, "DescribeOwner")
				if err == nil || !strings.Contains(err.Error(), "Collection is read-only") {
					t.Fatalf("namespaced=%v %s.%s: err=%v, want the uncatchable read-only fault", namespaced, target, mutation, err)
				}
			}
		}
	}
}
