package apextest

import (
	"crypto/sha256"
	"fmt"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRollupShapedWorkloadOutcome runs a synthetic rollup-shaped suite
// (Account and Contact only): a Contact trigger recomputes parent totals with
// aggregate SOQL, reaches objects through Schema.getGlobalDescribe, and runs
// many no-op updates. It prints an outcome digest so perf A/B runs can prove
// identical results. GLADE_ROLLUP_PERF_CLASSES sets the class count.
func TestRollupShapedWorkloadOutcome(t *testing.T) {
	classes := 12
	if raw := os.Getenv("GLADE_ROLLUP_PERF_CLASSES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			t.Fatalf("GLADE_ROLLUP_PERF_CLASSES=%q", raw)
		}
		classes = n
	}
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	write("force-app/main/classes/RollupEngine.cls", `
public class RollupEngine {
  public static Integer runs = 0;
  public static Boolean describes(String name) {
    return Schema.getGlobalDescribe().containsKey(name.toLowerCase());
  }
  public static void recalc(Set<Id> accountIds) {
    if (!describes('contact') || !describes('account')) { return; }
    runs++;
    Map<Id, Decimal> totals = new Map<Id, Decimal>();
    Map<Id, Integer> counts = new Map<Id, Integer>();
    for (AggregateResult ar : [SELECT AccountId a, COUNT(Id) c, SUM(Amount__c) s FROM Contact WHERE AccountId IN :accountIds GROUP BY AccountId]) {
      totals.put((Id) ar.get('a'), (Decimal) ar.get('s'));
      counts.put((Id) ar.get('a'), (Integer) ar.get('c'));
    }
    List<Account> upd = new List<Account>();
    for (Id aid : accountIds) {
      Decimal t = totals.containsKey(aid) ? totals.get(aid) : 0;
      Integer c = counts.containsKey(aid) ? counts.get(aid) : 0;
      upd.add(new Account(Id = aid, NumberOfEmployees = c, AnnualRevenue = t));
    }
    update upd;
  }
}
`)
	write("force-app/main/triggers/ContactRollup.trigger", `
trigger ContactRollup on Contact (after insert, after update, after delete) {
  Set<Id> ids = new Set<Id>();
  for (Contact c : (Trigger.isDelete ? Trigger.old : Trigger.new)) {
    if (c.AccountId != null) { ids.add(c.AccountId); }
  }
  RollupEngine.recalc(ids);
}
`)
	write("force-app/main/objects/Contact/fields/Amount__c.field-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amount__c</fullName><label>Amount</label><type>Number</type><precision>18</precision><scale>2</scale></CustomField>
`)
	for i := 0; i < classes; i++ {
		write(fmt.Sprintf("force-app/main/classes/RollupFlow%03dTest.cls", i), fmt.Sprintf(`
@isTest
private class RollupFlow%03dTest {
  static Account mk(Integer n) {
    Account a = new Account(Name = 'Acct%03d-' + n);
    insert a;
    return a;
  }
  @isTest static void insertsRollUp() {
    Account a = mk(1);
    List<Contact> cs = new List<Contact>();
    for (Integer i = 0; i < 20; i++) { cs.add(new Contact(LastName = 'C' + i, AccountId = a.Id, Amount__c = i + %d)); }
    insert cs;
    Account r = [SELECT NumberOfEmployees, AnnualRevenue FROM Account WHERE Id = :a.Id];
    System.assertEquals(20, r.NumberOfEmployees);
    System.assertEquals(190 + 20 * %d, r.AnnualRevenue);
  }
  @isTest static void noOpUpdates() {
    Account a = mk(2);
    Contact c = new Contact(LastName = 'N', AccountId = a.Id, Amount__c = 5);
    insert c;
    for (Integer i = 0; i < 25; i++) { update c; update a; }
    System.assertEquals(1, [SELECT COUNT() FROM Contact WHERE AccountId = :a.Id]);
    System.assertEquals(true, RollupEngine.runs >= 26);
  }
  @isTest static void deleteRollsDown() {
    Account a = mk(3);
    List<Contact> cs = new List<Contact>();
    for (Integer i = 0; i < 5; i++) { cs.add(new Contact(LastName = 'D' + i, AccountId = a.Id, Amount__c = 2)); }
    insert cs;
    delete cs[0];
    System.assertEquals(4, [SELECT NumberOfEmployees FROM Account WHERE Id = :a.Id].NumberOfEmployees);
  }
}
`, i, i, i, i))
	}
	index := loadRollupIndex(t, root)
	run := Run(index, Options{ParallelMethods: true, Parallelism: 2})
	summary := run.Summary()
	if summary.Total != classes*3 || summary.Passed != summary.Total {
		t.Fatalf("summary = %#v", summary)
	}
	var sb strings.Builder
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			fmt.Fprintf(&sb, "%s.%s=%v\n", c.ClassName, c.MethodName, c.Status)
		}
	}
	t.Logf("outcome-digest %x total=%d", sha256.Sum256([]byte(sb.String())), summary.Total)
}

func loadRollupIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}
