package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a named test never sees a criteria rule grant. Inside the inserting
// test, after an update into the rule, and after Test.stopTest, the reader's
// UserRecordAccess row is false and the Case has zero Rule CaseShare rows, and
// the reader's own Case query returns nothing. A test boundary does not force
// the recalculation the next transaction observes.
// testdata/conformance/criteria_sharing/grant_observations.tsv named-control rows 62.0 and 67.0.
//
// The anonymous checkpoint rows replay through glade exec against a persistent
// database in internal/gladecli/criteria_sharing_native_test.go.
func TestCriteriaSharingNamedTestNeverGrantsNative(t *testing.T) {
	const base = "testdata/conformance"
	rows := readNativeTSV(t, filepath.Join(base, "criteria_sharing", "grant_observations.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			want := ""
			for _, row := range rows {
				if row["case"] == "named-control" && row["api"] == api {
					want = row["observation"]
				}
			}
			if want == "" {
				t.Fatalf("no native named-control row for %s", api)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "case_phase", "project", "force-app"), filepath.Join(root, "force-app"), api)
			copyTree(t, filepath.Join(base, "case_phase", "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			class := "CriteriaGrantNamed" + strings.TrimSuffix(api, ".0")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), strings.ReplaceAll(criteriaGrantNamedClass, "CLASSNAME", class))
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("%s: expected one asserted observation: %+v", class, run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Actual: ")
			if got != want {
				t.Errorf("%s: Glade=%q native=%q (message %q)", class, got, want, run.Suites[0].Cases[0].Problem.Message)
			}
		})
	}
}

// The observation format is the native row's. M matches the rule when inserted,
// N never does, L is updated into the rule inside the test.
const criteriaGrantNamedClass = `@IsTest public class CLASSNAME {
 static String row(String tag, Id reader, Id caseId) {
  List<UserRecordAccess> access = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :reader AND RecordId = :caseId];
  return tag + ':ura=' + (access.isEmpty() ? 'no-row' : String.valueOf(access[0].HasReadAccess)) + ':rule=' + [SELECT COUNT() FROM CaseShare WHERE CaseId = :caseId AND RowCause = 'Rule'];
 }
 static String rows(Id reader, Id m, Id n, Id l) {
  return row('M', reader, m) + ',' + row('N', reader, n) + ',' + row('L', reader, l);
 }
 @IsTest static void observe() {
  Id profileId = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1].Id;
  String nonce = String.valueOf(DateTime.now().getTime());
  User reader = new User(Username = 'grant.reader.' + nonce + '@example.invalid', Alias = 'gread', Email = 'reader@example.invalid', LastName = 'GrantReader', ProfileId = profileId, TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
  insert reader;
  Group target = [SELECT Id FROM Group WHERE DeveloperName = 'CasePhaseReaders' AND Type = 'Regular' LIMIT 1];
  insert new GroupMember(GroupId = target.Id, UserOrGroupId = reader.Id);
  Case m = new Case(Subject = 'NATIVE_CASE_SHARE_PROBE', Status = 'New', Origin = 'Web');
  Case n = new Case(Subject = 'R450G_NOMATCH', Status = 'New', Origin = 'Web');
  Case l = new Case(Subject = 'R450G_LATE', Status = 'New', Origin = 'Web');
  System.runAs(new User(Id = UserInfo.getUserId())) {
   insert new List<Case>{m, n, l};
  }
  String afterSave = rows(reader.Id, m.Id, n.Id, l.Id);
  Test.startTest();
  l.Subject = 'NATIVE_CASE_SHARE_PROBE';
  System.runAs(new User(Id = UserInfo.getUserId())) {
   update l;
  }
  String inTest = rows(reader.Id, m.Id, n.Id, l.Id);
  Test.stopTest();
  String afterStop = rows(reader.Id, m.Id, n.Id, l.Id);
  Integer readerCases;
  System.runAs(reader) {
   readerCases = [SELECT COUNT() FROM Case WHERE Id IN :new Set<Id>{m.Id, n.Id, l.Id}];
  }
  System.assertEquals('EXPECTED', 'after-save=' + afterSave + '|in-test=' + inTest + '|after-stop=' + afterStop + '|readerCases=' + readerCases);
 }
}
`
