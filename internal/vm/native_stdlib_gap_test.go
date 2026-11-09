package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The API 68.0 rows in testdata/conformance/stdlib_gap_probe_c68 (captured
// 2026-11-06): Decimal.valueOf keeps 33 significant digits rounding HALF_EVEN,
// and Asia/Singapore formats as SGT / Singapore Time.
func TestExecNativeStdlibGapProbeRows(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "stdlib_gap_probe_c68", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 18 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	for _, row := range rows[1:] {
		kind, detail, _ := strings.Cut(row[1], ".")
		var source string
		switch {
		case kind == "decimal":
			plain, rest, _ := strings.Cut(row[2], "|")
			source = fmt.Sprintf(`Decimal d = Decimal.valueOf('%s');
System.assertEquals('%s', d.toPlainString());
System.assertEquals('%s', 'p=' + d.precision() + '|s=' + d.scale());`, detail, plain, rest)
		case kind == "sgt":
			source = fmt.Sprintf(`System.assertEquals('%s', Datetime.newInstanceGmt(2026, 1, 1, 0, 0, 0).format('yyyy-MM-dd HH:mm %s', 'Asia/Singapore'));`, row[2], detail)
		default:
			source = fmt.Sprintf(`System.assertEquals('%s', Datetime.newInstanceGmt(2026, 7, 1, 0, 0, 0).format('yyyy-MM-dd HH:mm z zzzz', 'Asia/Singapore'));`, row[2])
		}
		t.Run(row[1], func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The map key follows the SObject == projection: a queried record and a
// constructed record holding the same fields find each other, a key mutated
// after the put is missed, and Set<SObject> hashes the same way.
func TestExecSObjectKeyUsesEqualityProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
List<Account> queried = [SELECT Id, Name FROM Account WHERE Id = :a.Id];
Account constructed = new Account(Id = a.Id, Name = 'Acme');
System.assert(constructed == queried[0]);
Map<Account, Integer> byQueried = new Map<Account, Integer>{queried[0] => 1};
System.assertEquals(1, byQueried.get(constructed));
System.assert(byQueried.containsKey(constructed));
Map<Account, Integer> byConstructed = new Map<Account, Integer>{constructed => 2};
System.assertEquals(2, byConstructed.get(queried[0]));
Set<Account> members = new Set<Account>{queried[0]};
System.assert(members.contains(constructed));
members.add(constructed);
System.assertEquals(1, members.size());
Account same = new Account(Id = a.Id, Name = 'Acme');
Account other = new Account(Id = a.Id, Name = 'Other');
System.assertEquals(null, byQueried.get(other));
Account mutated = new Account(Name = 'before');
Set<Account> history = new Set<Account>();
history.add(mutated);
Map<Account, Integer> mutatedMap = new Map<Account, Integer>();
mutatedMap.put(mutated, 7);
mutated.Name = 'after';
System.assertEquals(null, mutatedMap.get(mutated));
System.assert(!history.contains(mutated));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
