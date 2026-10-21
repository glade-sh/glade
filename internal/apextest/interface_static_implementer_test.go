package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. A static method with typed-list parameters satisfies an interface instance method and runs through the interface, also when another class implements the interface.
func TestStaticMethodImplementsInterfaceMethod(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAI.cls"), `public class ZTAI { public interface BeforeUpdate { void beforeUpdate(List<SObject> newList, List<SObject> oldList); } }
`)
	writeFile(t, filepath.Join(root, base, "classes/ZStaticImpl.cls"), `public with sharing class ZStaticImpl implements ZTAI.BeforeUpdate {
    public static String seen = '';
    public static void beforeUpdate(List<Case> n, List<Case> o) { seen += 'static'; }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplS.cls"), `public class ZImplS implements ZTAI.BeforeUpdate {
    public static String seen = '';
    public void beforeUpdate(List<SObject> n, List<SObject> o) { seen += 'sobject'; }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplT.cls"), `public class ZImplT implements ZTAI.BeforeUpdate {
    public static String seen = '';
    public void beforeUpdate(List<Case> n, List<Case> o) { seen += 'typed'; }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZStaticProbe.cls"), `@isTest
public class ZStaticProbe {
  @isTest static void run() {
    String o = '';
    List<Case> lc = new List<Case>{new Case(Subject='x')};
    List<SObject> ls = new List<SObject>{new Case(Subject='y')};
    try { ((ZTAI.BeforeUpdate) new ZStaticImpl()).beforeUpdate(ls, ls); o += '\nSI|via-interface|ok seen=' + ZStaticImpl.seen; } catch (Exception e) { o += '\nSI|via-interface|' + e.getTypeName() + ' ' + e.getMessage(); }
    try { ZStaticImpl.beforeUpdate(lc, lc); o += '\nSI|direct-static|ok seen=' + ZStaticImpl.seen; } catch (Exception e) { o += '\nSI|direct-static|' + e.getTypeName() + ' ' + e.getMessage(); }
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ZTAI.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZStaticImpl.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplS.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZImplT.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, base, "classes/ZStaticProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
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
	want := `SI|via-interface|ok seen=static
SI|direct-static|ok seen=staticstatic`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}
