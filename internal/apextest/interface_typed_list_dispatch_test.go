package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. A class that implements an interface method with typed List<Case> parameters runs its own method, with or without another implementer that declares List<SObject>.
func TestInterfaceDispatchUsesReceiverClassForTypedListImplementers(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAI.cls"), `public class ZTAI { public interface BeforeUpdate { void beforeUpdate(List<SObject> newList, List<SObject> oldList); } }
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplT.cls"), `public class ZImplT implements ZTAI.BeforeUpdate {
    public static String seen = '';
    public void beforeUpdate(List<Case> n, List<Case> o) { seen += 'typed'; }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplS.cls"), `public class ZImplS implements ZTAI.BeforeUpdate {
    public static String seen = '';
    public void beforeUpdate(List<SObject> n, List<SObject> o) { seen += 'sobject'; }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAProbe.cls"), `@isTest
public class ZTAProbe {
  @isTest static void run() {
    String o = '';
    List<Case> lc = new List<Case>{new Case(Subject='x')};
    List<SObject> ls = new List<SObject>{new Case(Subject='y')};
    List<SObject> mixed = new List<SObject>(); mixed.add(new Case(Subject='z'));
    try { ((ZTAI.BeforeUpdate) new ZImplT()).beforeUpdate(lc, lc); o += '\nTA|typed-with-List<Case>|ok seen=' + ZImplT.seen; } catch (Exception e) { o += '\nTA|typed-with-List<Case>|' + e.getTypeName() + ' ' + e.getMessage(); }
    try { ((ZTAI.BeforeUpdate) new ZImplT()).beforeUpdate(ls, ls); o += '\nTA|typed-with-List<SObject>|ok seen=' + ZImplT.seen; } catch (Exception e) { o += '\nTA|typed-with-List<SObject>|' + e.getTypeName() + ' ' + e.getMessage(); }
    try { ((ZTAI.BeforeUpdate) new ZImplT()).beforeUpdate(mixed, mixed); o += '\nTA|typed-with-mixed|ok seen=' + ZImplT.seen; } catch (Exception e) { o += '\nTA|typed-with-mixed|' + e.getTypeName() + ' ' + e.getMessage(); }
    try { ((ZTAI.BeforeUpdate) new ZImplS()).beforeUpdate(lc, lc); o += '\nTA|sobject-with-List<Case>|ok seen=' + ZImplS.seen; } catch (Exception e) { o += '\nTA|sobject-with-List<Case>|' + e.getTypeName() + ' ' + e.getMessage(); }
    try { Object x = new ZImplT(); ((ZTAI.BeforeUpdate) x).beforeUpdate(ls, null); o += '\nTA|typed-null-old|ok'; } catch (Exception e) { o += '\nTA|typed-null-old|' + e.getTypeName() + ' ' + e.getMessage(); }
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAI.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplT.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplS.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	want := `TA|typed-with-List<Case>|ok seen=typed
TA|typed-with-List<SObject>|ok seen=typedtyped
TA|typed-with-mixed|ok seen=typedtypedtyped
TA|sobject-with-List<Case>|ok seen=sobject
TA|typed-null-old|ok`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}
