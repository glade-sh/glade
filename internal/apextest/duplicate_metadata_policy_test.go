package apextest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Coordinator contract: absent project sharing or field-denial metadata keeps
// the prior duplicate outcome. Explicit metadata takes precedence.
func duplicatePolicyProject(t testing.TB, api, source string, deny, private bool, grant ...bool) typesys.Index {
	t.Helper()
	root := t.TempDir()
	put := func(name, body string) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	put("sfdx-project.json", fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	const base = "force-app/main/default/"
	put(base+"classes/DuplicatePolicyTest.cls", source)
	put(base+"classes/DuplicatePolicyTest.cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	put(base+"objects/Contact/fields/Key__c.field-meta.xml", `<CustomField><fullName>Key__c</fullName><label>Key</label><type>Text</type><length>80</length></CustomField>`)
	for _, object := range []string{"Account", "Contact", "Lead"} {
		field := "Email"
		if object == "Account" {
			field = "Name"
		}
		if strings.Contains(source, "Key__c") {
			field = "Key__c"
			if object != "Contact" {
				continue
			}
		}
		put(base+"matchingRules/"+object+".matchingRule-meta.xml", fmt.Sprintf(`<MatchingRules><matchingRules><fullName>PolicyExact</fullName><label>Policy Exact</label><matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>%s</fieldName><matchingMethod>Exact</matchingMethod></matchingRuleItems><ruleStatus>Active</ruleStatus></matchingRules></MatchingRules>`, field))
		put(base+"duplicateRules/"+object+".PolicyExact.duplicateRule-meta.xml", fmt.Sprintf(`<DuplicateRule><actionOnInsert>Block</actionOnInsert><actionOnUpdate>Block</actionOnUpdate><alertText>Policy duplicate</alertText><duplicateRuleMatchRules><matchRuleSObjectType>%s</matchRuleSObjectType><matchingRule>PolicyExact</matchingRule></duplicateRuleMatchRules><isActive>true</isActive><masterLabel>Policy Exact</masterLabel><securityOption>EnforceSharingRules</securityOption><sortOrder>1</sortOrder></DuplicateRule>`, object))
		if private {
			put(base+"objects/"+object+"/"+object+".object-meta.xml", `<CustomObject><sharingModel>Private</sharingModel></CustomObject>`)
		}
	}
	if len(grant) > 0 && grant[0] {
		put(base+"profiles/Standard User.profile-meta.xml", `<Profile><fieldPermissions><field>Contact.Key__c</field><readable>true</readable><editable>true</editable></fieldPermissions></Profile>`)
	}
	if deny {
		put(base+"profiles/Admin.profile-meta.xml", `<Profile><fieldPermissions><field>Contact.Key__c</field><readable>false</readable><editable>false</editable></fieldPermissions></Profile>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	for _, d := range index.Diagnostics {
		if d.Severity == diagnostic.Error {
			t.Fatalf("project diagnostic: %+v", d)
		}
	}
	return index
}

func runDuplicatePolicy(t testing.TB, index typesys.Index, method string) {
	t.Helper()
	opts := Options{SelectedClasses: []string{"DuplicatePolicyTest"}, Filter: method, NoDiskCache: true, Parallelism: 1}
	cases := Discover(index, opts)
	if len(cases) != 1 {
		t.Fatalf("selected %d cases for %s", len(cases), method)
	}
	report := RunCasesContext(context.Background(), index, opts, cases)
	for _, suite := range report.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				t.Fatalf("%s: %+v", c.MethodName, c.Problem)
			}
		}
	}
}

const duplicateReaderSetup = `Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1]; User u=new User(LastName='Reader',Alias='reader',Email='reader@example.invalid',Username='reader@example.invalid',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u;`

func TestDuplicateUndeclaredSharingKeepsBaseline(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, seed, candidate string }{
			{"Account", "insert new Account(Name='Seed');", "new Account(Name='Seed')"},
			{"ContactParent", "Account a=new Account(Name='Parent');insert a;insert new Contact(LastName='Seed',AccountId=a.Id,Email='seed@example.invalid');", "new Contact(LastName='Candidate',AccountId=a.Id,Email='seed@example.invalid')"},
			{"ContactOrphan", "insert new Contact(LastName='Seed',Email='seed@example.invalid');", "new Contact(LastName='Candidate',Email='seed@example.invalid')"},
			{"Lead", "insert new Lead(LastName='Seed',Company='Synthetic',Email='seed@example.invalid');", "new Lead(LastName='Candidate',Company='Synthetic',Email='seed@example.invalid')"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := `@IsTest class DuplicatePolicyTest {@IsTest static void defaults(){` + duplicateReaderSetup + tc.seed + `System.runAs(u){Database.SaveResult result=Database.insert(` + tc.candidate + `,false);System.assertEquals(false,result.isSuccess());System.assertEquals(StatusCode.DUPLICATES_DETECTED,result.getErrors()[0].getStatusCode());}}}`
				runDuplicatePolicy(t, duplicatePolicyProject(t, api, source, false, false), "defaults")
			})
		}
	}
}

// NATIVE public-default Lead duplicate outcomes testdata/conformance/duplicate_public_defaults/org.tsv:2 istest 62.0
// NATIVE public-default Lead duplicate outcomes testdata/conformance/duplicate_public_defaults/org.tsv:3 istest 67.0
// Replay only the duplicate outcomes: the capture also checks SOQL visibility,
// whose undeclared-shape gap is outside this change.
func TestDuplicatePublicDefaultLeadNative(t *testing.T) {
	body, err := os.ReadFile("testdata/conformance/duplicate_public_defaults/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(rows) != 3 {
		t.Fatalf("want two native rows, got %d", len(rows)-1)
	}
	for _, line := range rows[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 || f[2] != "istest" || f[3] != "true" || f[4] != "Passed" || strings.TrimSpace(f[5]) != "publicDefault=Pass" {
			t.Fatalf("invalid native observation: %q", line)
		}
		t.Run(f[1], func(t *testing.T) {
			source := `@IsTest class DuplicatePolicyTest {@IsTest static void publicDefault(){` + duplicateReaderSetup + `
   insert new Lead(LastName='Seed',Company='Synthetic',Email='publiclead@example.invalid');
   System.runAs(u){
    Database.SaveResult fresh=Database.insert(new Lead(LastName='Fresh',Company='Synthetic',Email='fresh@example.invalid'),false);System.assert(fresh.isSuccess());
    Database.SaveResult dup=Database.insert(new Lead(LastName='Candidate',Company='Synthetic',Email='publiclead@example.invalid'),false);System.assertEquals(false,dup.isSuccess());System.assertEquals(StatusCode.DUPLICATES_DETECTED,dup.getErrors()[0].getStatusCode());
    Database.SaveResult blank=Database.insert(new Lead(LastName='Blank',Company='Synthetic'),false);System.assert(blank.isSuccess());
    Boolean blocked=false;try{insert new Lead(LastName='Statement',Company='Synthetic',Email='publiclead@example.invalid');}catch(DmlException e){blocked=e.getDmlStatusCode(0)=='DUPLICATES_DETECTED';}System.assert(blocked);
   }}}`
			runDuplicatePolicy(t, duplicatePolicyProject(t, f[1], source, false, false), "publicDefault")
		})
	}
}

func TestDuplicateAdminMatchingFieldMetadata(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, deny := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/deny=%t", api, deny), func(t *testing.T) {
				source := `@IsTest class DuplicatePolicyTest {@IsTest static void fields(){
 Database.SaveResult fresh=Database.insert(new Contact(LastName='Seed',Key__c='key'),false);
 System.assertEquals(EXPECT,fresh.isSuccess(),'first insert');
 if(fresh.isSuccess()){
 Database.SaveResult dup=Database.insert(new Contact(LastName='Duplicate',Key__c='key'),false);System.assertEquals(false,dup.isSuccess());System.assertEquals(StatusCode.DUPLICATES_DETECTED,dup.getErrors()[0].getStatusCode());
 Boolean blocked=false;try{insert new Contact(LastName='Statement',Key__c='key');}catch(DmlException e){blocked=e.getDmlStatusCode(0)=='DUPLICATES_DETECTED';}System.assert(blocked,'statement duplicate');
 }else{System.assertEquals(StatusCode.CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY,fresh.getErrors()[0].getStatusCode());}
 }}`
				expected := "true"
				if deny && api == "67.0" {
					expected = "false"
				}
				source = strings.ReplaceAll(source, "EXPECT", expected)
				index := duplicatePolicyProject(t, api, source, deny, false)
				runDuplicatePolicy(t, index, "fields")
			})
		}
	}
}

// The same 1,000 Contact insert workload and duplicate rejection run under both
// principals. Private metadata exercises access on the final matched record.
func BenchmarkDuplicateContactInsertThousand(b *testing.B) {
	benchmarkDuplicateContactInsertThousand(b, false)
}

func BenchmarkDuplicateContactKeyInsertThousand(b *testing.B) {
	benchmarkDuplicateContactInsertThousand(b, true)
}

func benchmarkDuplicateContactInsertThousand(b *testing.B, customField bool) {
	for _, runAs := range []bool{false, true} {
		name := "admin"
		if runAs {
			name = "runAs"
		}
		b.Run(name, func(b *testing.B) {
			setup, open, close := "", "", ""
			if runAs {
				setup = duplicateReaderSetup
				open = "System.runAs(u){"
				if customField {
					open += "System.assert(Contact.Key__c.getDescribe().isAccessible());"
				}
				close = "}"
			}
			source := `@IsTest class DuplicatePolicyTest {@IsTest static void workload(){` + setup + open + `
   for(Integer batch=0;batch<5;batch++){
    List<Contact> rows=new List<Contact>();
    for(Integer i=0;i<200;i++){rows.add(new Contact(LastName='Synthetic',Key__c='contact'+String.valueOf(batch*200+i)));}
    insert rows;
   }
   Database.SaveResult dup=Database.insert(new Contact(LastName='Synthetic',Key__c='contact42'),false);
   System.assertEquals(false,dup.isSuccess());System.assertEquals(StatusCode.DUPLICATES_DETECTED,dup.getErrors()[0].getStatusCode());
   System.assertEquals(1000,[SELECT count() FROM Contact]);` + close + `}}`
			if !customField {
				source = strings.ReplaceAll(source, "Key__c='contact'+String.valueOf(batch*200+i)", "Email='contact'+String.valueOf(batch*200+i)+'@example.invalid'")
				source = strings.ReplaceAll(source, "Key__c='contact42'", "Email='contact42@example.invalid'")
			}
			index := duplicatePolicyProject(b, "67.0", source, false, true, customField)
			runDuplicatePolicy(b, index, "workload")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runDuplicatePolicy(b, index, "workload")
			}
		})
	}
}
