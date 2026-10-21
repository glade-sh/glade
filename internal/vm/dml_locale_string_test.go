package vm

import "testing"

// PENDING-ROWS: the developer guide describes String locale values while the
// reference surface declares a LocaleOptions object. Preserve both payload
// shapes here without changing DML execution or the public type surface.
func TestDMLOptionsStringLocalePayload(t *testing.T) {
	for _, registered := range []bool{false, true} {
		machine := New(nil)
		if registered {
			if err := machine.RegisterClass(Class{Name: "Database.DMLOptions", PlatformType: true, Fields: map[string]Field{"LocaleOptions": {Name: "LocaleOptions", Type: "Database.LocaleOptions"}}}); err != nil {
				t.Fatal(err)
			}
		}
		program, err := CompileAnonymous(`Database.DMLOptions opts=new Database.DMLOptions(); System.assertEquals(null,opts.localeOptions);Database.LocaleOptions objectControl=new Database.LocaleOptions();opts.LocaleOptions=objectControl;System.assertEquals(objectControl,opts.localeOptions);opts.localeOptions=null;System.assertEquals(null,opts.LocaleOptions);opts.localeOptions='fr_CA';System.assertEquals('fr_CA',opts.LocaleOptions);Database.DMLOptions snapshot=opts.clone();System.assertEquals('fr_CA',snapshot.localeOptions);opts.localeOptions=null;System.assertEquals(null,opts.LocaleOptions);`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatalf("registered=%t: %v", registered, err)
		}
	}
}

func TestDMLOptionsCloneRegistersSplitBacking(t *testing.T) {
	machine := New(nil)
	options := newDatabaseDMLOptions()
	clone, _, _, handled, err := machine.callPlatformObjectMember(options, "clone", nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("DMLOptions.clone was not handled")
	}
	if clone.Ref != options.Ref || sameMapIdentity(clone.Fields, options.Fields) {
		t.Fatal("DMLOptions.clone must keep its Ref and copy its Fields backing")
	}
	if !machine.splitBacking(options.Ref) {
		t.Fatalf("DMLOptions.clone Ref %d was not registered for split-backing fallback", options.Ref)
	}
}
