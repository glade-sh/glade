package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Native rows: testdata/conformance/shapemisc_capture/org.tsv (isTest and
// anonymous routes, APIs 62.0, 67.0 and 68.0 equal 67.0) and acct-anonymous.tsv.
// The project replays custom-setting and null SObject member reads (csProbe,
// cs2Probe) and DML with formula and auto-number values present (fmProbe).

// shapemiscCarries are native rows Glade still differs on. Allowed Glade
// values are pinned so a change surfaces. All four are USER_MODE
// Database.insert or Database.upsert with allOrNone=false and a denied
// writable custom field: native returns a row error
// (CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY, message with a fieldNames suffix);
// Glade throws. The default-mode 67.0 dbInsertFalse_text row now matches
// through the admin field policy partial-failure path.
var shapemiscCarries = map[string]string{
	"67.0/fmProbe/dbInsertFalseUser_text": "EX System.DmlException|Operation failed due to fields being inaccessible on Sobject Probe_Item__c, check errors on Exception or Result! ",
	"62.0/fmProbe/dbInsertFalseUser_text": "EX System.DmlException|Operation failed due to fields being inaccessible on Sobject Probe_Item__c, check errors on Exception or Result! ",
	"67.0/fmProbe/dbUpsertFalseUser_text": "EX System.DmlException|Operation failed due to fields being inaccessible on Sobject Probe_Item__c, check errors on Exception or Result! ",
	"62.0/fmProbe/dbUpsertFalseUser_text": "EX System.DmlException|Operation failed due to fields being inaccessible on Sobject Probe_Item__c, check errors on Exception or Result! ",
}

var shapemiscCaseLine = regexp.MustCompile(`^[a-z]\d\d_|^(insert|db|upsert|update)`)

func readShapemiscTSV(t *testing.T, path string) [][]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n")[1:] {
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

func TestShapemiscNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "shapemisc_capture")
	rows := readShapemiscTSV(t, filepath.Join(dir, "org.tsv"))
	acct := readShapemiscTSV(t, filepath.Join(dir, "acct-anonymous.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			for _, rel := range []string{"sfdx-project.json"} {
				body, err := os.ReadFile(filepath.Join(project, rel))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
			}
			classes := filepath.Join(project, "force-app/main/default")
			err := filepath.Walk(classes, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(project, path)
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			probe := filepath.Join(root, "force-app/main/default/classes/AcctProbeTest.cls")
			writeFile(t, probe, `@IsTest private class AcctProbeTest { @IsTest static void acctProbe() {
AcctHolder h = new AcctHolder();
List<String> out = new List<String>();
try { out.add('d01_acctChainId=' + h.acct.Id); } catch (Exception e) { out.add('d01_acctChainId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d02_acctChainOwnerId=' + h.acct.OwnerId); } catch (Exception e) { out.add('d02_acctChainOwnerId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d03_acctChainName=' + h.acct.Name); } catch (Exception e) { out.add('d03_acctChainName=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d04_acctChainCreatedDate=' + h.acct.CreatedDate); } catch (Exception e) { out.add('d04_acctChainCreatedDate=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d05_acctChainIsDeleted=' + h.acct.IsDeleted); } catch (Exception e) { out.add('d05_acctChainIsDeleted=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d06_acctChainWrite=' + (h.acct.Name = 'x')); } catch (Exception e) { out.add('d06_acctChainWrite=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d07_acctChainParentId=' + h.acct.ParentId); } catch (Exception e) { out.add('d07_acctChainParentId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d08_acctChainParentName=' + h.acct.Parent.Name); } catch (Exception e) { out.add('d08_acctChainParentName=EX ' + e.getTypeName() + '|' + e.getMessage()); }
System.assertEquals('', 'OBSERVED\n' + String.join(out, '\n')); } }`)
			writeFile(t, probe+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			observed := map[string]string{}
			for _, method := range []struct{ class, name string }{
				{"ShapeProbeTest", "csProbe"}, {"ShapeProbeTest", "cs2Probe"}, {"ShapeProbeTest", "fmProbe"}, {"AcctProbeTest", "acctProbe"},
			} {
				run := Run(index, Options{SelectedClasses: []string{method.class}, SelectedMethod: method.name, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("%s: expected one observation: %+v", method.name, run.Summary())
				}
				for _, line := range strings.Split(run.Suites[0].Cases[0].Problem.Message, "\n") {
					if shapemiscCaseLine.MatchString(line) {
						key, value, _ := strings.Cut(line, "=")
						observed[method.name+"/"+key] = value
					}
				}
			}
			checked := 0
			for _, row := range rows {
				if row[0] != api {
					continue
				}
				checked++
				got, ok := observed[row[1]+"/"+row[2]]
				if !ok {
					t.Errorf("%s %s: Glade produced no observation", row[1], row[2])
					continue
				}
				if got == row[3] {
					continue
				}
				if want, carried := shapemiscCarries[api+"/"+row[1]+"/"+row[2]]; carried && got == want {
					continue
				}
				t.Errorf("%s %s:\n native: %s\n Glade:  %s", row[1], row[2], row[3], got)
			}
			if checked != 109 {
				t.Fatalf("expected 109 native rows at %s, found %d", api, checked)
			}
			for _, row := range acct {
				if row[0] != api {
					continue
				}
				if got := observed["acctProbe/"+row[1]]; got != row[2] {
					t.Errorf("%s:\n native: %s\n Glade:  %s", row[1], row[2], got)
				}
			}
		})
	}
}
