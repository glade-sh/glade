package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: testdata/conformance/alias_slot_view/native (API 62.0 and 67.0
// agree). A reference written back into a field after an alias update keeps the
// field's declared type, so overload resolution on a later read still picks the
// field's type and not the type of the variable that was mutated.
func aliasSlotViewNativeRows(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/alias_slot_view/native/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("bad native row %q", line)
		}
		rows[parts[0]] = parts[1]
	}
	return rows
}

func TestAliasUpdateKeepsDeclaredFieldViewNativeConformance(t *testing.T) {
	rows := aliasSlotViewNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			add := func(name, body string) {
				writeFile(t, filepath.Join(classes, name+".cls"), body)
				writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			add("C27bRoot", `public virtual class C27bRoot { public Integer count = 0; }`)
			add("C27bFin", `public virtual class C27bFin {
  public String tag(C27bRoot r) { return 'root'; }
  public String tag(C27bFull r) { return 'full'; }
}`)
			add("C27bFull", `public virtual class C27bFull extends C27bRoot {
  public C27bFull post;
  public C27bFin fin;
  public C27bFull() {}
  public C27bFull(C27bFull post) { this.post = post; }
  public String viaField() { return this.fin.tag(this.post); }
}`)
			add("C27bHolder", `public class C27bHolder {
  public static List<C27bFull> cache = new List<C27bFull>();
  public static C27bFull held;
  public static void bump(C27bRoot r) { r.count++; }
}`)
			add("C27bSHold", `public class C27bSHold {
  public static C27bFull s;
  public static void bump(C27bRoot r) { r.count++; }
  public static String viaStatic(C27bFin fin) { return fin.tag(s); }
}`)
			add("C27bViewTest", `@IsTest private class C27bViewTest {
  @IsTest static void rows() {
    C27bFin fin = new C27bFin();
    C27bFull r = new C27bFull();
    C27bFull f = new C27bFull(r);
    f.fin = fin;
    System.assertEquals(`+conformanceApexString(rows["plain"])+`, f.viaField(), 'plain');

    C27bHolder.cache.add(f);
    C27bRoot x = r;
    x.count = 5;
    System.assertEquals(`+conformanceApexString(rows["assignViaSupertypeVar"])+`, f.viaField() + '|' + r.count, 'assignViaSupertypeVar');

    C27bHolder.bump(r);
    System.assertEquals(`+conformanceApexString(rows["mutateViaSupertypeParam"])+`, f.viaField() + '|' + r.count, 'mutateViaSupertypeParam');

    C27bHolder.held = f;
    C27bRoot y = f.post;
    y.count = 9;
    System.assertEquals(`+conformanceApexString(rows["staticHolderAssign"])+`, C27bHolder.held.viaField() + '|' + r.count, 'staticHolderAssign');

    List<C27bFull> local = new List<C27bFull>{ f };
    for (C27bRoot z : new List<C27bRoot>{ r }) { z.count = 11; }
    System.assertEquals(`+conformanceApexString(rows["loopOverSupertypeList"])+`, local[0].viaField() + '|' + r.count, 'loopOverSupertypeList');

    C27bFull nofin = new C27bFull(r);
    String got;
    try { nofin.viaField(); got = 'none'; }
    catch (Exception e) { got = e.getTypeName() + '|' + e.getMessage(); }
    System.assertEquals(`+conformanceApexString(rows["nullReceiver"])+`, got, 'nullReceiver');
  }

  // A static field that is itself the aliased object keeps its declared type.
  @IsTest static void staticRows() {
    C27bFin fin = new C27bFin();
    C27bFull r = new C27bFull();
    C27bSHold.s = r;
    System.assertEquals(`+conformanceApexString(rows["staticPlain"])+`, C27bSHold.viaStatic(fin), 'staticPlain');
    C27bRoot x = C27bSHold.s;
    x.count = 5;
    System.assertEquals(`+conformanceApexString(rows["staticAssignViaSupertypeVar"])+`, C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count, 'staticAssignViaSupertypeVar');
    C27bSHold.bump(C27bSHold.s);
    System.assertEquals(`+conformanceApexString(rows["staticMutateViaSupertypeParam"])+`, C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count, 'staticMutateViaSupertypeParam');
    C27bFull r2 = new C27bFull();
    C27bSHold.s = r2;
    C27bRoot y = r2;
    y.count = 9;
    System.assertEquals(`+conformanceApexString(rows["staticAssignViaLocalAlias"])+`, C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count, 'staticAssignViaLocalAlias');
    for (C27bRoot z : new List<C27bRoot>{ r2 }) { z.count = 11; }
    System.assertEquals(`+conformanceApexString(rows["staticLoopOverSupertypeList"])+`, C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count, 'staticLoopOverSupertypeList');
  }
}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}
