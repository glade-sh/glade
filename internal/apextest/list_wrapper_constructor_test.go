package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestListWrapperConstructorPreservesElements(t *testing.T) {
	for _, tc := range []struct {
		name, field   string
		helper        bool
		overload      bool
		interfaceList bool
		duplicate     bool
		getter        bool
	}{
		{name: "shadowedField", field: "records"},
		{name: "helperTraversal", field: "records", helper: true},
		{name: "overloadedConstructor", field: "records", helper: true, overload: true},
		{name: "distinctField", field: "items", helper: true, overload: true},
		{name: "getterTraversal", field: "records", helper: true, overload: true, getter: true},
		{name: "interfaceList", field: "records", helper: true, overload: true, interfaceList: true, getter: true},
		{name: "duplicateSource", field: "records", helper: true, overload: true, duplicate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			namespace := ""
			if tc.duplicate {
				namespace = "samplepkg"
			}
			project := newSalesforceSurfaceProject(t, fmt.Sprintf(`{"packageDirectories":[{"path":"library","package":"Sample Library"},{"path":"force-app","default":true}],"sourceApiVersion":"66.0","namespace":%q}`, namespace))
			helperCall := ""
			if tc.helper {
				helperCall = "this.objectType = this.resolve(records);"
			}
			overload := ""
			if tc.overload {
				overload = fmt.Sprintf(`private Bundle(Iterable<Id> ids) {
  for (Id recordId : ids) { this.%s.add(new Entry(new Account(Id=recordId))); }
}`, tc.field)
			}
			entryType, entryImplements, iface := "Entry", "", ""
			if tc.interfaceList {
				entryType, entryImplements, iface = "Item", "implements Item", "public interface Item { SObject get(); }"
			}
			getter := "return this." + tc.field + ";"
			if tc.getter {
				getter = fmt.Sprintf("for (%s item : this.%s) { this.touch(item.get()); } return this.%s;", entryType, tc.field, tc.field)
			}
			source := fmt.Sprintf(`public class BundleProbe {
  public static Bundle wrap(List<SObject> records) { return new Bundle(records); }
  %s
  public class Entry %s {
    public SObject record;
    public Entry(SObject record) { this.record = record; }
    public SObject get() { return this.record; }
  }
  public class Bundle {
    private List<%s> %s = new List<%s>();
    public SObjectType objectType;
    private Bundle(List<SObject> records) {
      %s
      for (SObject record : records) { this.%s.add(new Entry(record)); }
    }
    %s
    private SObjectType resolve(List<SObject> records) {
      SObjectType found;
      for (SObject record : records) { found = record.getSObjectType(); }
      return found;
    }
    public List<%s> get() { %s }
    private void touch(SObject record) { this.objectType = record.getSObjectType(); }
  }
}`, iface, entryImplements, entryType, tc.field, entryType, helperCall, tc.field, overload, entryType, getter)
			project.writeClass("BundleProbe", source)
			if tc.duplicate {
				librarySource := strings.Replace(source, "public class BundleProbe {", "public class BundleProbe { public static String origin() { return 'library'; }", 1)
				writeFile(t, filepath.Join(project.root, "library/main/default/classes/BundleProbe.cls"), librarySource)
			}
			project.writeClass("BundleProbeTest", `@IsTest private class BundleProbeTest {
  @IsTest static void typedSource() {
    Account first = new Account(Name='first');
    Account second = new Account(Name='second');
    List<Account> records = new List<Account>{first, second};
    BundleProbe.Bundle bundle = BundleProbe.wrap(records);
    System.assertEquals(2, bundle.get().size());
    System.assertEquals(2, records.size());
    System.assert(bundle.get()[0].get() === first);
    System.assert(bundle.get()[1].get() === second);
  }
  @IsTest static void genericSource() {
    List<SObject> records = new List<SObject>{new Account(Name='first'), new Account(Name='second')};
    BundleProbe.Bundle bundle = BundleProbe.wrap(records);
    System.assertEquals(2, bundle.get().size());
    System.assertEquals(2, records.size());
    System.assertEquals('first', bundle.get()[0].get().get('Name'));
    System.assertEquals('second', bundle.get()[1].get().get('Name'));
  }
}`)
			run := project.run()
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				for _, suite := range run.Suites {
					for _, test := range suite.Cases {
						if test.Problem != nil {
							t.Logf("%s: %#v", test.Name, *test.Problem)
						}
					}
				}
				t.Fatalf("summary = %#v suites = %#v", got, run.Suites)
			}
		})
	}
}
