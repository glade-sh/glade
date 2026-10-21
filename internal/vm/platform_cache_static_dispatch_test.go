package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Existing owned cache observations cover named partitions and synchronous
// Session availability. A passive catalog declaration must use that runtime too.
func TestCachePassiveStaticDeclarationsUseCapturedRuntime(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, scope := range []string{"Cache.Org", "Cache.Session"} {
			t.Run(api+"/"+scope, func(t *testing.T) {
				machine := cacheStaticDispatchMachine(api)
				method := cacheStaticDispatchDeclaration(scope, "getPartition", "String")
				partition, err := machine.callMethod(method, []Value{String("PcAlt")}, &Result{})
				if err != nil {
					t.Fatal(err)
				}
				if partition.Type != scope+"Partition" || partition.Fields["name"].Text != "local.PcAlt" || partition.Fields["scope"].Text != scope {
					t.Fatalf("partition = %#v", partition)
				}
			})
		}
		for _, named := range []bool{false, true} {
			route := "anonymous"
			if named {
				route = "istest"
			}
			t.Run(api+"/availability/"+route, func(t *testing.T) {
				machine := cacheStaticDispatchMachine(api)
				if named {
					machine.EnableTestContext()
				}
				available, err := machine.callMethod(cacheStaticDispatchDeclaration("Cache.Session", "isAvailable", ""), nil, &Result{})
				if err != nil || available.Kind != ValueBool || available.Bool != !named {
					t.Fatalf("availability = %v, %v", available, err)
				}
			})
		}
	}
}

func TestCacheStaticDispatchRetainsRejectionAndSourceControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ scope, key, exception, message string }{
			{"Cache.Org", "", "cache.InvalidParamException", "Partition name cannot be null or empty and must be alphanumeric"},
			{"Cache.Org", "bad-name", "cache.Org.OrgCacheException", "Invalid partition: partition name must be alphanumeric."},
			{"Cache.Org", "Missing", "cache.Org.OrgCacheException", "Invalid partition - partition 'local.Missing' does not exist."},
			{"Cache.Session", "Missing", "cache.Session.SessionCacheException", "Invalid partition: partition 'local.Missing' does not exist."},
		} {
			t.Run(api+"/"+tc.scope+"/"+tc.key, func(t *testing.T) {
				_, err := cacheStaticDispatchMachine(api).call(tc.scope+".getPartition", []Value{String(tc.key)}, nil, &Result{})
				var issue *apexThrowError
				if !errors.As(err, &issue) || issue.value.Type != tc.exception || issue.value.Fields["message"].Text != tc.message {
					t.Fatalf("rejection = %v", err)
				}
			})
			t.Run(api+"/passive/"+tc.scope+"/"+tc.key, func(t *testing.T) {
				_, err := cacheStaticDispatchMachine(api).callMethod(cacheStaticDispatchDeclaration(tc.scope, "getPartition", "String"), []Value{String(tc.key)}, &Result{})
				var issue *apexThrowError
				if !errors.As(err, &issue) || issue.value.Type != tc.exception || issue.value.Fields["message"].Text != tc.message {
					t.Fatalf("passive rejection = %v", err)
				}
			})
		}
		t.Run(api+"/unsupported", func(t *testing.T) {
			_, err := cacheStaticDispatchMachine(api).callMethod(cacheStaticDispatchDeclaration("Cache.Session", "unobserved", ""), nil, &Result{})
			var issue *apexThrowError
			if !errors.As(err, &issue) || issue.value.Type != "UnsupportedOperationException" {
				t.Fatalf("uncaptured method = %v", err)
			}
		})
		t.Run(api+"/source", func(t *testing.T) {
			program, err := CompileAnonymous("return 'source';")
			if err != nil {
				t.Fatal(err)
			}
			method := cacheStaticDispatchDeclaration("Cache.Org", "getPartition", "String")
			method.Modifiers, method.ReturnType, method.Program = nil, "String", program
			value, err := cacheStaticDispatchMachine(api).callMethod(method, []Value{String("PcAlt")}, &Result{})
			if err != nil || value.Text != "source" {
				t.Fatalf("source method = %v, %v", value, err)
			}
		})
	}
}

func TestCacheStaticDispatchRetainsSignatureBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, shape := range []string{"nonstatic", "integerParameter"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				method := cacheStaticDispatchDeclaration("Cache.Org", "getPartition", "String")
				args := []Value{String("PcAlt")}
				if shape == "nonstatic" {
					method.IsStatic = false
				} else {
					method.Params[0].Type = "Integer"
					args = []Value{Int(1)}
				}
				_, err := cacheStaticDispatchMachine(api).callMethod(method, args, &Result{})
				var issue *apexThrowError
				if !errors.As(err, &issue) || issue.value.Type != "UnsupportedOperationException" {
					t.Fatalf("unobserved declaration = %v", err)
				}
			})
		}
	}
}

func TestCacheStaticDispatchRetainsMissingDefault(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, passive := range []bool{false, true} {
			route := "builtin"
			if passive {
				route = "passive"
			}
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := cacheStaticDispatchMachine(api)
				partitions := machine.Org.Objects["PlatformCachePartition"]
				partitions.Records = map[storage.ID]storage.Record{}
				machine.Org.Objects["PlatformCachePartition"] = partitions
				var err error
				if passive {
					_, err = machine.callMethod(cacheStaticDispatchDeclaration("Cache.Session", "isAvailable", ""), nil, &Result{})
				} else {
					_, err = machine.call("Cache.Session.isAvailable", nil, nil, &Result{})
				}
				var issue *apexThrowError
				if !errors.As(err, &issue) || issue.value.Type != "cache.Session.SessionCacheException" || issue.value.Fields["message"].Text != "A default partition was not found.  To cache without partition reference, designate a partition as default." {
					t.Fatalf("missing-default rejection = %v", err)
				}
			})
		}
	}
}

func cacheStaticDispatchDeclaration(scope, member, parameter string) Method {
	method := Method{Name: scope + "." + member, ClassName: scope, IsStatic: true, Modifiers: []string{"passive-generated"}, ReturnType: "Boolean"}
	if parameter != "" {
		method.Params = []Param{{Name: "name", Type: parameter}}
		method.ReturnType = scope + "Partition"
	}
	return method
}

func cacheStaticDispatchMachine(api string) *VM {
	org := storage.NewOrgState()
	org.APIVersion = api
	storage.EnsureStandardObject(&org, "PlatformCachePartition")
	partitions := org.Objects["PlatformCachePartition"]
	for i, name := range []string{"PcMain", "PcAlt"} {
		id := storage.ID("0Pp00000000000" + string(rune('1'+i)))
		partitions.Records[id] = storage.Record{ID: id, Object: "PlatformCachePartition", Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue(name), "NamespacePrefix": storage.StringValue(""), "IsDefaultPartition": storage.BooleanValue(i == 0),
		}}
	}
	org.Objects["PlatformCachePartition"] = partitions
	machine := New(nil)
	machine.SetOrg(&org)
	return machine
}
