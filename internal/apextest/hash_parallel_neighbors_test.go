package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Shapes next to the SF-1 and SF-2 hash and Decimal rows keep their earlier
// behavior. The fixes are gated to the captured shapes: Julian civil dates
// before 1582-10-15 in the Datetime builders, the three captured zones before
// 2008, a Decimal list ==, an insert past a Percent or Number integer limit,
// and an insert of a DateTime outside 1700-01-01..4000-12-31. Each neighbor
// below stays on the base side of its gate.
// neighborZones names the cases that run in a zone other than Los Angeles.
var neighborZones = map[string]string{"a zone without history parses 1920 as before": "America/New_York"}

func zoneOf(zone string) string {
	if zone == "" {
		return "America/Los_Angeles"
	}
	return zone
}

func TestHashParallelNeighborsKeepBase(t *testing.T) {
	family := filepath.Join("testdata", "conformance", "surf_collections_strings", "project")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	err := filepath.WalkDir(family, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(family, path)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch {
		case strings.HasSuffix(path, ".cls"):
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", rel), string(body))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", rel+"-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		case rel == "glade.yml":
			writeFile(t, filepath.Join(root, rel), string(body))
		default:
			writeFile(t, filepath.Join(root, "force-app/main/default", rel), string(body))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A class with its own equals() keeps deciding List and Set equality.
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/NbW.cls"), `public class NbW { public Decimal d; public NbW(Decimal x) { d = x; } public Boolean equals(Object o) { return o instanceof NbW && ((NbW) o).d == d; } public Integer hashCode() { return 1; } }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/NbW.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	index := loadTestIndex(t, root)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: func() *storage.OrgState {
		o := capturedFixtureOrg(t, index, "67.0")
		return &o
	}()})
	for _, test := range []struct{ name, source, want string }{
		{"cutover and later civil dates", `
System.debug(Datetime.newInstanceGmt(Date.newInstance(1582, 10, 15), Time.newInstance(0, 0, 0, 0)).getTime());
System.debug(Datetime.valueOfGmt('2000-01-01 00:00:00').getTime());
System.debug(Datetime.newInstanceGmt(Date.newInstance(1900, 1, 1), Time.newInstance(0, 0, 0, 0)).getTime());`,
			"-12219292800000\n946684800000\n-2208988800000"},
		{"zone neighbors", `
System.debug(Datetime.newInstance(Date.newInstance(2012, 7, 1), Time.newInstance(12, 0, 0, 0)).getTime());
System.debug(Datetime.newInstance(Date.newInstance(2012, 1, 1), Time.newInstance(12, 0, 0, 0)).getTime());`,
			"1341169200000\n1325448000000"},
		{"datetime arithmetic and other builders keep the en_US term", `
System.debug(Datetime.newInstance(0L).addDays(1).hashCode());
System.debug(Datetime.valueOfGmt('1970-01-01 00:00:00').hashCode());
System.debug(Date.newInstance(1970, 1, 2).hashCode() + ' ' + Time.newInstance(0, 0, 1, 0).hashCode());`,
			"86936795\n577755\n86400000 1000"},
		{"pre-cutover builders and readers agree", `
Datetime a = Datetime.newInstance(1500, 1, 1, 12, 0, 0);
System.debug(a.day() + ' ' + a.format('yyyy-MM-dd HH:mm') + ' ' + a.date());
Datetime g = Datetime.newInstanceGmt(1500, 1, 1, 0, 0, 0);
System.debug(g.formatGmt('yyyy-MM-dd') + ' ' + g.dateGmt() + ' ' + (g.getTime() == Datetime.valueOfGmt('1500-01-01 00:00:00').getTime()));
System.debug(Datetime.parse('1/1/1700, 12:00 PM').getTime() == Datetime.newInstance(1700, 1, 1, 12, 0, 0).getTime());
System.debug(Datetime.valueOf('1500-01-01 12:00:00').getTime() == a.getTime());
System.debug(Date.newInstance(0, 1, 1));`,
			"1 1500-01-01 12:00 1500-01-01 00:00:00\n1500-01-01 1500-01-01 00:00:00 true\ntrue\ntrue\n0000-01-01 00:00:00"},
		{"class instances with equals() ignore Decimal scale", `
List<NbW> a = new List<NbW>{new NbW(1.5)}; List<NbW> b = new List<NbW>{new NbW(1.50)};
System.debug(a == b);
System.debug(a.equals(b));
System.debug(a.contains(new NbW(1.50)));
System.debug(a.indexOf(new NbW(1.50)));
System.debug(new Set<NbW>{new NbW(1.5)} == new Set<NbW>{new NbW(1.50)});`,
			"true\ntrue\ntrue\n0\ntrue"},
		{"a zone without history parses 1920 as before", `
System.debug(Datetime.parse('3/20/1920, 12:00 PM').getTime());`,
			"-1571036400000"},
		{"the cutover instant reads in Julian labels in a negative offset zone", `
Datetime d = Datetime.newInstanceGmt(1582, 10, 15, 4, 0, 0);
System.debug(d.format('yyyy-MM-dd HH:mm'));
System.debug(d.day() + ' ' + d.year());`,
			"1582-10-04 20:00\n4 1582"},
		{"list equality neighbors", `
System.debug(new List<Double>{1.5} == new List<Double>{1.50});
System.debug(new List<Decimal>{1.5} == new List<Decimal>{1.5});
System.debug(new List<Integer>{1, 2} == new List<Integer>{1, 2});
System.debug(new List<Decimal>{1.5} == new List<Decimal>{2.5});
Decimal a = 1.5; Decimal b = 1.50;
System.debug(a == b);`,
			"true\ntrue\ntrue\nfalse\ntrue"},
		{"number and datetime neighbors", `
Dec_Probe__c ok = new Dec_Probe__c(Pct__c = 9.99, Curr__c = 123456.789, Num5__c = 123456.789, Num8__c = 99.99999999);
insert ok;
System.debug('insert ok');
ok.Pct__c = 123456.789;
update ok;
System.debug('update past the limit is unchanged');
Hc__c early = new Hc__c(Dt__c = Datetime.newInstanceGmt(Date.newInstance(1700, 1, 1), Time.newInstance(0, 0, 0, 0)), D__c = Date.newInstance(4000, 12, 31));
insert early;
System.debug('range edge and Date field ok');
Hc__c later = new Hc__c(Dt__c = Datetime.newInstanceGmt(Date.newInstance(2000, 1, 1), Time.newInstance(0, 0, 0, 0)));
insert later;
later.Dt__c = Datetime.newInstanceGmt(Date.newInstance(4000, 12, 31), Time.newInstance(0, 0, 0, 0));
update later;
System.debug('datetime update at the bound');`,
			"insert ok\nupdate past the limit is unchanged\nrange edge and Date field ok\ndatetime update at the bound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if analysis := sema.AnalyzeAnonymous(index, test.source, "67.0"); analysis.HasErrors() {
				t.Fatalf("compile: %v", analysis.Diagnostics)
			}
			configure := func(machine *vm.VM) {
				if err := machine.ConfigureTestEnvironment(config.TestConfig{User: config.TestUserConfig{Locale: "en_US", TimeZone: zoneOf(neighborZones[test.name]), Language: "en_US"}}); err != nil {
					t.Fatal(err)
				}
			}
			got := runSurfAnonymous(t, runner, index, test.source, "67.0", configure)
			if got != test.want {
				t.Fatalf("want:\n%s\ngot:\n%s", test.want, got)
			}
		})
	}
}
