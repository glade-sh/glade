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
							{"put('private', Account.SObjectType)", "beforeSize + 1"},
							{"put('account', User.SObjectType)", "beforeSize"},
							{"putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType})", "beforeSize + 1"},
							{"remove('account')", "beforeSize - 1"}, {"clear()", "0"},
							{"keySet().remove('account')", "beforeSize"},
							{"keySet().clear()", "beforeSize"},
							{"keySet().removeAll(new Set<String>{'account'})", "beforeSize"},
							{"keySet().retainAll(new Set<String>{'user'})", "beforeSize"},
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
