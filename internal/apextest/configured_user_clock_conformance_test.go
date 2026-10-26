package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const calendarFixture = "testdata/conformance/configured_user_clock"

func calendarSettingsFixture(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(filepath.Join(calendarFixture, "project"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join(calendarFixture, "project"), path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data := string(raw)
		if strings.HasSuffix(path, ".cls-meta.xml") {
			data = strings.ReplaceAll(data, "67.0", api)
		}
		writeFile(t, filepath.Join(root, rel), data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	return root
}

func TestConfiguredUserClockNativeConformance(t *testing.T) {
	f, err := os.Open(filepath.Join(calendarFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("native rows=%d, want 8", len(rows)-1)
	}
	for _, r := range rows[1:] {
		var values []string
		if err := json.Unmarshal([]byte(r[2]), &values); err != nil {
			t.Fatal(err)
		}
		modes := []string{"configured", "runAs"}
		if r[1] == "adminDefaults" {
			modes = []string{"unset"}
		}
		for _, mode := range modes {
			t.Run(r[0]+"/"+r[1]+"/"+mode, func(t *testing.T) {
				root := calendarSettingsFixture(t, r[0])
				clock := strings.Replace(values[0], " ", "T", 1) + "Z"
				settings := fmt.Sprintf("test:\n  clock:\n    start: '%s'\n", clock)
				if mode == "configured" {
					settings += fmt.Sprintf("  user:\n    timeZone: %s\n    locale: %s\n    language: %s\n    defaultCurrencyIsoCode: %s\n", values[1], values[2], values[3], values[4])
				} else {
					settings += "  user:\n    timeZone: America/Los_Angeles\n    locale: en_US\n"
				}
				if mode == "unset" {
					settings = ""
				}
				writeFile(t, filepath.Join(root, "glade.yml"), settings)
				path := filepath.Join(root, "force-app/main/default/classes/CalendarSettingsObservation.cls")
				raw, _ := os.ReadFile(path)
				source := string(raw)
				if mode == "configured" {
					source = strings.ReplaceAll(source, "System.runAs(user('Europe/Berlin'))", "")
					source = strings.ReplaceAll(source, "System.runAs(user('GMT'))", "")
					source = strings.ReplaceAll(source, "System.runAs(u)", "")
				}
				source = strings.ReplaceAll(source, "System.assert(false,'CALENDAR_ROW|'+", "System.assertEquals('"+r[2]+"',")
				writeFile(t, path, source)
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"CalendarSettingsObservation"}, SelectedMethod: r[1], NoDiskCache: true})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					data, _ := json.Marshal(run)
					t.Fatalf("native %s: %s", r[2], data)
				}
			})
		}
	}
}

func TestConfiguredUserClockIsolation(t *testing.T) {
	root := calendarSettingsFixture(t, "67.0")
	// Source stays identical across settings changes so compiled runtime reuse is exercised.
	source := `@IsTest private class CalendarSettingsObservation {
  static String initialized=String.valueOf(Date.today());
  @TestSetup static void setup(){System.assertEquals(String.valueOf(Date.today()),initialized);}
  @IsTest static void defaultsFirst(){check(false);}
  @IsTest static void defaultsSecond(){check(false);}
  @IsTest static void configuredFirst(){check(true);}
  @IsTest static void configuredSecond(){check(true);}
  static void check(Boolean configured){
   System.assertEquals(configured?'Europe/Berlin':'America/Los_Angeles',UserInfo.getTimeZone().getID());
   System.assertEquals(configured?'2026-05-05':'2026-05-02',String.valueOf(Date.today()));
   System.assertEquals(configured?'2026-05-05':'2026-05-02',initialized);
   System.assertEquals(configured?'2026-05-04 22:30:00':'2026-05-02 12:00:00',Datetime.now().formatGmt('yyyy-MM-dd HH:mm:ss'));
   System.assertEquals(configured?'EUR':'USD',UserInfo.getDefaultCurrency());
   System.assertEquals(configured?'de_DE':'en_US',UserInfo.getLocale());
   System.assertEquals(configured?'de':'en_US',UserInfo.getLanguage());
   User current=[SELECT TimeZoneSidKey,LocaleSidKey,LanguageLocaleKey,DefaultCurrencyIsoCode FROM User WHERE Id=:UserInfo.getUserId()];
   System.assertEquals(UserInfo.getTimeZone().getID(),current.TimeZoneSidKey);
   System.assertEquals(UserInfo.getLocale(),current.LocaleSidKey);
   System.assertEquals(UserInfo.getLanguage(),current.LanguageLocaleKey);
   if(configured)System.assertEquals('EUR',current.DefaultCurrencyIsoCode);
   insert new List<CalendarObservation__c>{new CalendarObservation__c(Name='Current',Day__c=Date.today()),new CalendarObservation__c(Name='Tomorrow',Day__c=Date.today().addDays(1)),new CalendarObservation__c(Name='Following',Day__c=Date.today().addDays(7))};
   System.assertEquals(configured?2:1,[SELECT COUNT() FROM CalendarObservation__c WHERE Day__c=THIS_WEEK]);
   System.assertEquals(configured?1:2,[SELECT COUNT() FROM CalendarObservation__c WHERE Day__c=NEXT_WEEK]);
  }
 }`
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CalendarSettingsObservation.cls"), source)
	for _, tc := range []struct{ settings, filter string }{
		{"", "defaults"},
		{"test:\n  user:\n    timeZone: Europe/Berlin\n    locale: de_DE\n    language: de\n    defaultCurrencyIsoCode: EUR\n  clock:\n    start: '2026-05-05T00:30:00+02:00'\n", "configured"},
		{"", "defaults"},
	} {
		writeFile(t, filepath.Join(root, "glade.yml"), tc.settings)
		index := loadTestIndex(t, root)
		for i := 0; i < 2; i++ {
			run := Run(index, Options{Filter: tc.filter})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
				data, _ := json.Marshal(run)
				t.Fatalf("settings %q run %d: %s", tc.settings, i, data)
			}
		}
	}
}
