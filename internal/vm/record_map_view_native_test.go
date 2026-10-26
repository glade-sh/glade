package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native record_map_views/org.tsv captures the helper-return route at both APIs.
func TestRecordMapViewNativeTypeTokenRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named-isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				helper, err := CompileAnonymousWithOptions(`return (List<Contact>)rows;`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "RecordMapViews", APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "RecordMapViews.contacts", ClassName: "RecordMapViews", IsStatic: true, ReturnType: "List<Contact>", Params: []Param{{Name: "rows", Type: "List<SObject>"}}, Program: helper, APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				tokenHelper, err := CompileAnonymousWithOptions(`return rows.getSObjectType();`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "RecordMapViews.token", ClassName: "RecordMapViews", IsStatic: true, ReturnType: "Schema.SObjectType", Params: []Param{{Name: "rows", Type: "List<Contact>"}}, Program: tokenHelper, APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
List<SObject> raw=new List<SObject>{new Account(Id='001000000000001AAA',Name='Meadow')};
List<Contact> view=RecordMapViews.contacts(raw);
Object token=view.getSObjectType();
Object inlineToken=((List<Contact>)raw).getSObjectType();
Object relayToken=RecordMapViews.token(view);
List<Contact> matching=RecordMapViews.contacts(new List<SObject>{new Contact(Id='003000000000001AAA',LastName='Brook')});
Object matchingToken=matching.getSObjectType();
List<Contact> empty=RecordMapViews.contacts(new List<SObject>());
Object emptyToken=empty.getSObjectType();
List<SObject> concrete=new List<Contact>{new Contact(LastName='Concrete')};
System.assertEquals(Contact.SObjectType,concrete.getSObjectType());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if route == "anonymous" {
					_, err = machine.Execute(program)
				} else {
					_, err = machine.ExecuteTestInClass(program, "RecordMapViews")
				}
				if err != nil {
					t.Fatal(err)
				}

				for _, name := range []string{"token", "inlineToken", "relayToken", "matchingToken", "emptyToken"} {
					if got := machine.Globals[name]; got.Kind != ValueNull {
						t.Errorf("%s=%s; native null", name, got.String())
					}
				}
				for _, name := range []string{"view", "matching", "empty"} {
					view := machine.Globals[name]
					t.Logf("%s type=%s static=%s runtime=%s generic=%t", name, view.Type, view.Static, view.Runtime, view.genericSObjectList)
					got, _, _, handled, err := callListStdlibMember(view, "getSObjectType", nil)
					if !handled || err != nil || got.Kind != ValueNull {
						t.Errorf("%s stdlib token=%s handled=%t err=%v; native null", name, got.String(), handled, err)
					}
					got, handled, err = machine.callListValueMember(name, view, "getSObjectType", nil, &Result{})
					if !handled || err != nil || got.Kind != ValueNull {
						t.Errorf("%s schema dispatch token=%s handled=%t err=%v; native null", name, got.String(), handled, err)
					}
				}
			})
		}
	}
}

// Native rows distinguish constructors from putAll for both supported key types.
func TestRecordMapViewNativePutAllRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named-isTest"} {
			for _, keyType := range []string{"Id", "String"} {
				t.Run(api+"/"+route+"/"+keyType, func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					storage.EnsureStandardObject(&org, "Contact")
					machine.SetOrg(&org)
					program, err := CompileAnonymousWithOptions(`
List<SObject> raw=new List<SObject>{new Account(Id='001000000000001AAA',Name='Meadow')};
List<Contact> view=(List<Contact>)raw;
Map<`+keyType+`,Contact> constructed=new Map<`+keyType+`,Contact>(view);
System.assertEquals(1,constructed.size());
System.assertEquals(true,constructed.containsKey('001000000000001AAA'));
Map<`+keyType+`,Contact> appended=new Map<`+keyType+`,Contact>();
Boolean rejected=false;
try { appended.putAll(view); } catch(TypeException ex) { rejected=true; }
System.assertEquals(true,rejected,'native putAll rejects the actual Account record');
System.assertEquals(0,appended.size());
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "RecordMapViews", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
					if route == "anonymous" {
						_, err = machine.Execute(program)
					} else {
						_, err = machine.ExecuteTestInClass(program, "RecordMapViews")
					}
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
