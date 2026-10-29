package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native F001 and R012 at APIs 62/67: unqualified source metadata becomes
// qualified custom paths. Pin the shared constructor and both execution routes.
func TestNamespacedFieldSetNativeUnitEntryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"unit", "anonymous", "isTest"} {
			for _, qualified := range []bool{false, true} {
				name := "localMetadata"
				if qualified {
					name = "qualifiedMetadata"
				}
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					org := testDataOrg()
					org.Namespace, org.APIVersion = "nsx", api
					definition := storage.ObjectDefinition{APIName: "nsx__Thing__c", Fields: map[string]storage.Field{
						"Name":          {APIName: "Name", Label: "Name", Type: storage.FieldString},
						"nsx__Size__c":  {APIName: "nsx__Size__c", Label: "Size", Type: storage.FieldDecimal},
						"nsx__Owner__c": {APIName: "nsx__Owner__c", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "nsx__Owner__r"},
					}}
					org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
					paths := []string{"Size__c", "Owner__r.Name", "Name"}
					if qualified {
						paths[0], paths[1] = "nsx__Size__c", "nsx__Owner__r.Name"
					}
					fields := make([]storage.FieldSetMemberMetadata, 0, len(paths))
					for _, path := range paths {
						fields = append(fields, storage.FieldSetMemberMetadata{Field: path})
					}
					org.Metadata.FieldSets = []storage.FieldSetMetadata{{ObjectName: "Thing__c", Name: "Shown", Fields: fields}}
					machine := New(nil)
					machine.SetOrg(&org)
					if route == "unit" {
						for i, want := range []string{"nsx__Size__c", "nsx__Owner__r.Name", "Name"} {
							got := machine.fieldSetMemberValue(definition.APIName, definition, fields[i])
							if got.Fields["fieldPath"].Text != want {
								t.Errorf("field %s: got %s want %s", paths[i], got.Fields["fieldPath"].Text, want)
							}
						}
						return
					}
					source := `List<String> paths = new List<String>();
for (Schema.FieldSetMember member : Schema.SObjectType.Thing__c.FieldSets.Shown.getFields()) paths.add(member.getFieldPath());
System.assertEquals('nsx__Size__c|nsx__Owner__r.Name|Name',String.join(paths,'|'));`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if route == "isTest" {
						machine.EnableTestContext()
						if registerErr := machine.RegisterClass(Class{Name: "NativeFieldSetEntry", Namespace: "nsx", IsTest: true}); registerErr != nil {
							t.Fatal(registerErr)
						}
						_, err = machine.ExecuteTestInClass(program, "nsx.NativeFieldSetEntry")
					} else {
						_, err = machine.Execute(program)
					}
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// R005/R006 pin scalar and generic cast targets. System aliases and an
// ordinary unnamespaced source class are controls on the same label function.
func TestNamespacedCastNativeUnitTargetRoutes(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{{Name: "TypeTextAnchor", Namespace: "nsx"}, {Name: "LocalAnchor"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ target, want string }{
		{"nsx.TypeTextAnchor", "nsx.TypeTextAnchor"},
		{"List<nsx.TypeTextAnchor>", "List<nsx.TypeTextAnchor>"},
		{"LocalAnchor", "LocalAnchor"},
		{"Pattern", "System.Pattern"},
	} {
		t.Run(row.target, func(t *testing.T) {
			if got := machine.typeExceptionDisplayName(row.target); got != row.want {
				t.Fatalf("got %q want %q", got, row.want)
			}
		})
	}
}

// R001/R002 show the provider's declared static list alongside its instance
// fields. Direct, nested and explicit/generic proxy rendering share the helper.
func TestNamespacedProviderNativeUnitDisplayRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace, org.APIVersion = "nsx", api
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{Name: "TypeTextStubProvider", Namespace: "nsx", Interfaces: []string{"StubProvider"},
				Fields:       map[string]Field{"providerFlag": {Name: "providerFlag", Type: "Boolean"}, "providerName": {Name: "providerName", Type: "String"}},
				StaticFields: map[string]Field{"observedTypes": {Name: "observedTypes", Type: "List<String>", Static: true, Value: List(), InitialValue: List()}},
			}); err != nil {
				t.Fatal(err)
			}
			provider := Object("nsx.TypeTextStubProvider")
			provider.Fields["providerFlag"], provider.Fields["providerName"] = Bool(true), String("synthetic")
			proxy := Object("nsx.TypeTextStubApi")
			proxy.Fields["__gladeStubProvider"], proxy.Fields["__gladeStubbedType"] = provider, String("nsx.TypeTextStubApi")
			want := "TypeTextStubProvider:[observedTypes=(), providerFlag=true, providerName=synthetic]"
			for route, got := range map[string]string{
				"directProvider": machine.defaultObjectDisplayString(provider),
				"nestedProvider": machine.nestedDisplayString(provider, make(map[uint64]bool)),
				"genericProxy":   machine.stubProxyDisplayString(proxy, make(map[uint64]bool)),
				"explicitProxy":  machine.stubProxyDisplayStringForOperation(proxy, make(map[uint64]bool), true),
			} {
				t.Run(route, func(t *testing.T) {
					expected := want
					if strings.Contains(route, "Proxy") {
						expected = "TypeTextStubApi__sfdc_ApexStub:[stubProvider=" + want + "]"
					}
					if got != expected {
						t.Fatalf("got %q want %q", got, expected)
					}
				})
			}
			if len(provider.Fields) != 2 {
				t.Fatal("displaying static state changed the provider instance")
			}
			t.Run("staticOnlyNilInstanceMap", func(t *testing.T) {
				provider.Fields = nil
				if got := machine.nestedDisplayString(provider, make(map[uint64]bool)); got != "TypeTextStubProvider:[observedTypes=()]" {
					t.Fatalf("static-only provider: %q", got)
				}
			})

		})
	}
}

// R001/R002 capture a provider that implements StubProvider directly, with no
// superclass. No native row covers a provider that extends another class, so
// the static-state display stays limited to the captured shape: an inherited
// provider keeps the instance-field text it had before the port, on the
// direct, nested and stub-proxy routes, at APIs 62/67. Base-preserving guard
// pending a native capture of the inherited shape.
func TestNamespacedInheritedProviderKeepsInstanceDisplayRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace, org.APIVersion = "nsx", api
			machine.SetOrg(&org)
			for _, class := range []Class{
				{Name: "TypeTextBaseProvider", Namespace: "nsx", Modifiers: []string{"virtual"}, Interfaces: []string{"StubProvider"},
					StaticFields: map[string]Field{"baseTypes": {Name: "baseTypes", Type: "List<String>", Static: true, Value: List(), InitialValue: List()}}},
				{Name: "TypeTextChildProvider", Namespace: "nsx", SuperClass: "TypeTextBaseProvider",
					Fields:       map[string]Field{"childFlag": {Name: "childFlag", Type: "Boolean"}},
					StaticFields: map[string]Field{"childTypes": {Name: "childTypes", Type: "List<String>", Static: true, Value: List(), InitialValue: List()}}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			provider := Object("nsx.TypeTextChildProvider")
			provider.Fields["childFlag"] = Bool(true)
			proxy := Object("nsx.TypeTextStubApi")
			proxy.Fields["__gladeStubProvider"], proxy.Fields["__gladeStubbedType"] = provider, String("nsx.TypeTextStubApi")
			want := "TypeTextChildProvider:[childFlag=true]"
			for route, got := range map[string]string{
				"directProvider": machine.defaultObjectDisplayString(provider),
				"nestedProvider": machine.nestedDisplayString(provider, make(map[uint64]bool)),
				"genericProxy":   machine.stubProxyDisplayString(proxy, make(map[uint64]bool)),
				"explicitProxy":  machine.stubProxyDisplayStringForOperation(proxy, make(map[uint64]bool), true),
			} {
				t.Run(route, func(t *testing.T) {
					expected := want
					if strings.Contains(route, "Proxy") {
						expected = "TypeTextStubApi__sfdc_ApexStub:[stubProvider=" + want + "]"
					}
					if got != expected {
						t.Fatalf("got %q want %q", got, expected)
					}
				})
			}
		})
	}
}

// R008-R011: owner-side keySet/get/containsKey must agree for the custom type
// and its generated share. Qualified and foreign-key controls share the path.
func TestNamespacedGlobalDescribeNativeUnitLookupRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				org := storage.NewOrgState()
				org.Namespace, org.APIVersion = "nsx", api
				for _, name := range []string{"nsx__TypeTextItem__c", "nsx__TypeTextItem__Share", "nsx__TypeTextItem__ChangeEvent"} {
					org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
				}
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "TypeTextEntry", Namespace: "nsx", IsTest: true}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymous(`Map<String,Schema.SObjectType> allTypes=Schema.getGlobalDescribe();
System.assertEquals(false,allTypes.containsKey('TypeTextItem__c'));
System.assertEquals(null,allTypes.get('TypeTextItem__c'));
System.assertEquals(false,allTypes.containsKey('TypeTextItem__Share'));
System.assertEquals(null,allTypes.get('TypeTextItem__Share'));
System.assertEquals(true,allTypes.containsKey('nsx__TypeTextItem__c'));
System.assertNotEquals(null,allTypes.get('NSX__TYPETEXTITEM__C'));
System.assertEquals(null,allTypes.get('other__TypeTextItem__c'));
System.assertEquals(false,allTypes.keySet().contains('typetextitem__share'));
System.assertEquals(true,allTypes.keySet().contains('nsx__typetextitem__share'));
System.assertEquals(true,allTypes.keySet().contains('nsx__typetextitem__changeevent'));
System.assertEquals(true,allTypes.containsKey('nsx__TypeTextItem__ChangeEvent'));
System.assertEquals(false,allTypes.containsKey('TypeTextItem__ChangeEvent'));
System.assertEquals(null,allTypes.get('TypeTextItem__ChangeEvent'));
System.assertEquals('nsx__TypeTextItem__ChangeEvent',allTypes.get('nsx__TypeTextItem__ChangeEvent').getDescribe().getName());`)
				if err != nil {
					t.Fatal(err)
				}
				machine.SetCurrentNamespace("nsx")
				if route == "isTest" {
					machine.EnableTestContext()
					_, err = machine.ExecuteTestInClass(program, "nsx.TypeTextEntry")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatal(err)
				}
				// Keep the separate app/module map contract out of global-only filtering.
				raw := machine.schemaGlobalDescribe()
				if _, ok := raw.Map[mapKey(String("typetextitem__share"))]; !ok {
					t.Fatal("global projection mutated raw app/module share aliases")
				}
				for _, operation := range []string{"Schema.getAppDescribe('app')", "Schema.getModuleDescribe('module')"} {
					control, compileErr := CompileAnonymous("Map<String,Schema.SObjectType> raw=" + operation + ";System.assertNotEquals(null,raw.get('TypeTextItem__Share'));System.assert(raw.keySet().contains('TypeTextItem__Share')); ")
					if compileErr != nil {
						t.Fatal(compileErr)
					}
					if _, executeErr := machine.Execute(control); executeErr != nil {
						t.Fatal(executeErr)
					}
				}
			})
		}
	}
}

// R008-R011 and T002/T003 also apply when an app/module read primes the raw
// cache before the owning namespace is bound to the execution entry.
func TestNamespacedGlobalDescribeWarmRawCacheNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route+"/warmAppModuleCache", func(t *testing.T) {
				machine := New(nil)
				org := storage.NewOrgState()
				org.Namespace, org.APIVersion = "nsx", api
				for _, name := range []string{"nsx__TypeTextItem__c", "nsx__TypeTextItem__Share"} {
					org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
				}
				machine.SetOrg(&org)
				if route == "isTest" {
					// Warm the raw cache after test schema initialization, while
					// the caller namespace is still deliberately unbound.
					machine.EnableTestContext()
				}
				raw := machine.schemaGlobalDescribe()
				rawSize := len(raw.Map)
				if _, ok := raw.Map[mapKey(String("TypeTextItem__c"))]; !ok {
					t.Fatal("unbound app/module cache lost its local custom-object alias")
				}
				machine.SetCurrentNamespace("nsx")
				program, err := CompileAnonymousWithOptions(`
Map<String,Schema.SObjectType> types = Schema.getGlobalDescribe();
System.assertEquals(null, types.get('TypeTextItem__c'));
System.assertEquals(null, types.get('TypeTextItem__Share'));
System.assertEquals(null, types.get('TypeTextItem__ChangeEvent'));
System.assertEquals(false, types.containsKey('TypeTextItem__ChangeEvent'));
System.assertEquals(false, types.keySet().contains('typetextitem__c'));
System.assertEquals(false, types.keySet().contains('typetextitem__share'));
System.assertEquals(false, types.keySet().contains('typetextitem__changeevent'));
System.assertEquals(true, types.keySet().contains('nsx__typetextitem__changeevent'));
System.assertEquals('nsx__TypeTextItem__ChangeEvent', types.get('nsx__TypeTextItem__ChangeEvent').getDescribe().getName());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if route == "isTest" {
					if err := machine.RegisterClass(Class{Name: "WarmDescribeEntry", Namespace: "nsx", IsTest: true, APIVersion: api}); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, err = machine.ExecuteTestInClass(program, "nsx.WarmDescribeEntry")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := machine.schemaGlobalDescribe(); len(got.Map) != rawSize {
					t.Fatalf("global projection changed raw cache size: %d -> %d", rawSize, len(got.Map))
				}
				if _, ok := raw.Map[mapKey(String("TypeTextItem__Share"))]; !ok {
					t.Fatal("global projection removed the cached app/module alias")
				}
			})
		}
	}
}
