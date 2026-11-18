package apextest

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testreport"
)

// Each method returns a digest of what one schema-dependent route answers and
// reports it through a failing assertion, so the run report carries the
// answer. The lazy spread must give the answers the eager spread gives.
const lazySpreadParityClass = `@IsTest
private class LazySpreadParityTest {
	static String md5(String s) { return EncodingUtil.convertToHex(Crypto.generateDigest('MD5', Blob.valueOf(s))); }
	static void report(String s) { System.assert(false, s); }
	@IsTest static void globalDescribe() {
		Map<String, Schema.SObjectType> all = Schema.getGlobalDescribe();
		List<String> keys = new List<String>(all.keySet()); keys.sort();
		report(all.size() + ' ' + md5(String.join(keys, ',')) + ' ' + all.containsKey('quote') + ' ' + all.containsKey('network') + ' ' + all.containsKey('asyncapexjob'));
	}
	@IsTest static void describeEveryObject0() {
		List<String> keys = new List<String>(Schema.getGlobalDescribe().keySet()); keys.sort();
		List<String> names = new List<String>(); List<String> prefixes = new List<String>(); List<String> labels = new List<String>();
		List<String> fields = new List<String>(); List<String> rtis = new List<String>(); List<String> kids = new List<String>();
		Integer position = 0;
		for (String k : keys) {
			if (Math.mod(position++, 4) != 0) { continue; }
			Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get(k).getDescribe();
			names.add(d.getName()); prefixes.add(d.getName() + d.getKeyPrefix()); labels.add(d.getName() + d.getLabel());
			fields.add(d.getName() + d.fields.getMap().size()); rtis.add(d.getName() + d.getRecordTypeInfos().size()); kids.add(d.getName() + d.getChildRelationships().size());
		}
		report(keys.size() + ' ' + md5(String.join(names, ',')) + ' ' + md5(String.join(prefixes, ',')) + ' ' + md5(String.join(labels, ',')) + ' ' + md5(String.join(fields, ',')) + ' ' + md5(String.join(rtis, ',')) + ' ' + md5(String.join(kids, ',')));
	}
	@IsTest static void describeEveryObject1() {
		List<String> keys = new List<String>(Schema.getGlobalDescribe().keySet()); keys.sort();
		List<String> names = new List<String>(); List<String> prefixes = new List<String>(); List<String> labels = new List<String>();
		List<String> fields = new List<String>(); List<String> rtis = new List<String>(); List<String> kids = new List<String>();
		Integer position = 0;
		for (String k : keys) {
			if (Math.mod(position++, 4) != 1) { continue; }
			Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get(k).getDescribe();
			names.add(d.getName()); prefixes.add(d.getName() + d.getKeyPrefix()); labels.add(d.getName() + d.getLabel());
			fields.add(d.getName() + d.fields.getMap().size()); rtis.add(d.getName() + d.getRecordTypeInfos().size()); kids.add(d.getName() + d.getChildRelationships().size());
		}
		report(keys.size() + ' ' + md5(String.join(names, ',')) + ' ' + md5(String.join(prefixes, ',')) + ' ' + md5(String.join(labels, ',')) + ' ' + md5(String.join(fields, ',')) + ' ' + md5(String.join(rtis, ',')) + ' ' + md5(String.join(kids, ',')));
	}
	@IsTest static void describeEveryObject2() {
		List<String> keys = new List<String>(Schema.getGlobalDescribe().keySet()); keys.sort();
		List<String> names = new List<String>(); List<String> prefixes = new List<String>(); List<String> labels = new List<String>();
		List<String> fields = new List<String>(); List<String> rtis = new List<String>(); List<String> kids = new List<String>();
		Integer position = 0;
		for (String k : keys) {
			if (Math.mod(position++, 4) != 2) { continue; }
			Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get(k).getDescribe();
			names.add(d.getName()); prefixes.add(d.getName() + d.getKeyPrefix()); labels.add(d.getName() + d.getLabel());
			fields.add(d.getName() + d.fields.getMap().size()); rtis.add(d.getName() + d.getRecordTypeInfos().size()); kids.add(d.getName() + d.getChildRelationships().size());
		}
		report(keys.size() + ' ' + md5(String.join(names, ',')) + ' ' + md5(String.join(prefixes, ',')) + ' ' + md5(String.join(labels, ',')) + ' ' + md5(String.join(fields, ',')) + ' ' + md5(String.join(rtis, ',')) + ' ' + md5(String.join(kids, ',')));
	}
	@IsTest static void describeEveryObject3() {
		List<String> keys = new List<String>(Schema.getGlobalDescribe().keySet()); keys.sort();
		List<String> names = new List<String>(); List<String> prefixes = new List<String>(); List<String> labels = new List<String>();
		List<String> fields = new List<String>(); List<String> rtis = new List<String>(); List<String> kids = new List<String>();
		Integer position = 0;
		for (String k : keys) {
			if (Math.mod(position++, 4) != 3) { continue; }
			Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get(k).getDescribe();
			names.add(d.getName()); prefixes.add(d.getName() + d.getKeyPrefix()); labels.add(d.getName() + d.getLabel());
			fields.add(d.getName() + d.fields.getMap().size()); rtis.add(d.getName() + d.getRecordTypeInfos().size()); kids.add(d.getName() + d.getChildRelationships().size());
		}
		report(keys.size() + ' ' + md5(String.join(names, ',')) + ' ' + md5(String.join(prefixes, ',')) + ' ' + md5(String.join(labels, ',')) + ' ' + md5(String.join(fields, ',')) + ' ' + md5(String.join(rtis, ',')) + ' ' + md5(String.join(kids, ',')));
	}
	@IsTest static void describeSelected() {
		List<String> rows = new List<String>();
		for (Schema.DescribeSObjectResult d : Schema.describeSObjects(new List<String>{'ApexClass', 'EmailTemplate', 'Pricebook2', 'Entitlement', 'ServiceContract', 'OpportunityLineItem'})) {
			rows.add(d.getName() + '|' + d.getKeyPrefix() + '|' + d.getLabelPlural() + '|' + d.fields.getMap().size() + '|' + d.isCreateable());
		}
		report(String.join(rows, ';') + ' ' + Schema.SObjectType.Task.fields.WhatId.getDescribe().getReferenceTo().size());
	}
	@IsTest static void childRelationships() {
		report(Schema.SObjectType.Account.getChildRelationships().size() + ' ' + Schema.SObjectType.Opportunity.getChildRelationships().size() + ' ' + Schema.SObjectType.Contact.getChildRelationships().size() + ' ' + Schema.SObjectType.User.getChildRelationships().size());
	}
	@IsTest static void soqlColdObjects() {
		report([SELECT COUNT() FROM AsyncApexJob] + ' ' + [SELECT COUNT() FROM CronTrigger] + ' ' + [SELECT COUNT() FROM OrgWideEmailAddress] + ' ' + [SELECT COUNT() FROM Entitlement] + ' ' + [SELECT COUNT() FROM ServiceContract] + ' ' + [SELECT COUNT() FROM Solution]);
	}
	@IsTest static void soqlRelationshipsAndSubqueries() {
		report([SELECT Id, (SELECT Id FROM OpportunityLineItems) FROM Opportunity].size() + ' ' + [SELECT Id, (SELECT Id FROM GroupMembers) FROM Group].size() + ' ' +
			[SELECT COUNT() FROM Account WHERE Id IN (SELECT AccountId FROM Entitlement)] + ' ' + [SELECT COUNT() FROM Contact WHERE Id NOT IN (SELECT ContactId FROM ServiceContract)] + ' ' +
			[SELECT TYPEOF What WHEN Account THEN Name WHEN Solution THEN SolutionName END FROM Task].size());
	}
	@IsTest static void dynamicSoql() {
		report(Database.query('SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Entitlement WHERE Id IN (SELECT EntitlementId FROM ServiceContract))').size() + ' ' + Database.countQuery('SELECT COUNT() FROM Contact WHERE Id NOT IN (SELECT ContactId FROM ServiceContract)') + ' ' + Database.query('SELECT Id FROM Pricebook2').size());
	}
	@IsTest static void dmlOnColdObjects() {
		Group g = new Group(Name = 'Lazy', Type = 'Regular'); insert g;
		insert new GroupMember(GroupId = g.Id, UserOrGroupId = UserInfo.getUserId());
		Account a = new Account(Name = 'A'); insert a;
		Entitlement e = new Entitlement(Name = 'E', AccountId = a.Id); insert e;
		report([SELECT COUNT() FROM GroupMember WHERE GroupId = :g.Id] + ' ' + String.valueOf(g.Id).substring(0, 3) + ' ' + String.valueOf(e.Id).substring(0, 3) + ' ' + [SELECT COUNT() FROM Entitlement WHERE AccountId = :a.Id]);
		delete a;
	}
	@IsTest static void keyPrefixLookups() {
		report(((Id) '550000000000001AAA').getSObjectType() + ' ' + ((Id) '810000000000001AAA').getSObjectType() + ' ' + ((Id) '0DM000000000001AAA').getSObjectType() + ' ' + ((Id) '707000000000001AAA').getSObjectType() + ' ' + ((Id) '0Hn000000000001AAA').getSObjectType());
	}
	// Most catalog objects have no catalog key prefix. The complete spread gives
	// them the next free custom prefix in name order, so a cold object's prefix
	// is known only through the spread's unique assignment.
	@IsTest static void reassignedPrefixLookups() {
		report(((Id) 'a00000000000001AAA').getSObjectType() + ' ' + ((Id) 'a0E000000000001AAA').getSObjectType() + ' ' + ((Id) 'a0Z000000000001AAA').getSObjectType() + ' ' + ((Id) 'a1A000000000001AAA').getSObjectType() + ' ' + ((Id) 'a00000000000001').getSObjectType());
	}
	@IsTest static void reassignedPrefixDmlAndPolymorphic() {
		List<String> rows = new List<String>();
		Database.DeleteResult one = Database.delete('a00000000000001AAA', false);
		rows.add(one.isSuccess() + ' ' + one.getErrors()[0].getStatusCode());
		List<Database.DeleteResult> many = Database.delete(new List<Id>{'a0E000000000001AAA', 'a0Z000000000001AAA'}, false);
		rows.add(many.size() + ' ' + many[0].getErrors()[0].getStatusCode() + ' ' + many[1].getErrors()[0].getStatusCode());
		try { Task t = new Task(Subject = 'T', WhatId = 'a00000000000001AAA'); insert t; rows.add('insert ' + [SELECT What.Type FROM Task].size()); } catch (Exception e) { rows.add(e.getTypeName() + ' ' + e.getMessage()); }
		try { SObject s = ((Id) 'a0E000000000001AAA').getSObjectType().newSObject('a0E000000000001AAA'); rows.add(String.valueOf(s.getSObjectType()) + ' ' + s.Id); } catch (Exception e) { rows.add(e.getTypeName() + ' ' + e.getMessage()); }
		try { Database.update(new List<SObject>{((Id) 'a00000000000001AAA').getSObjectType().newSObject('a00000000000001AAA')}, false); rows.add('update'); } catch (Exception e) { rows.add(e.getTypeName() + ' ' + e.getMessage()); }
		report(String.join(rows, ' | '));
	}
	// The virtual schema objects list every object the org holds, cold ones too.
	@IsTest static void entityDefinitionSoslAndSoql() {
		List<List<SObject>> found = [FIND 'Work Access' IN ALL FIELDS RETURNING EntityDefinition(QualifiedApiName)];
		List<String> rows = new List<String>();
		for (SObject o : found[0]) { rows.add(String.valueOf(o.get('QualifiedApiName'))); }
		rows.sort();
		report(String.join(rows, ',') + ' ' + [SELECT COUNT() FROM EntityDefinition] + ' ' + [SELECT COUNT() FROM EntityDefinition WHERE QualifiedApiName = 'WorkAccess'] + ' ' + [SELECT COUNT() FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'WorkAccess'] + ' ' + [SELECT COUNT() FROM EntityParticle WHERE EntityDefinition.QualifiedApiName = 'WorkAccess']);
	}
	@IsTest static void typeAndInstanceLookups() {
		SObject s = (SObject) Type.forName('Schema.Entitlement').newInstance();
		report(String.valueOf(Type.forName('AsyncApexJob')) + ' ' + String.valueOf(Type.forName('ServiceResource')) + ' ' + s.getSObjectType() + ' ' + new AsyncApexJob().getSObjectType() + ' ' + Schema.getGlobalDescribe().get('ServiceResource').newSObject().getSObjectType());
	}
	@IsTest static void unknownAndGated() {
		String quote; String network; String curr;
		try { quote = String.valueOf([SELECT COUNT() FROM Quote]); } catch (QueryException e) { quote = e.getMessage(); }
		try { network = String.valueOf([SELECT COUNT() FROM Network]); } catch (QueryException e) { network = e.getMessage(); }
		try { curr = String.valueOf([SELECT COUNT() FROM CurrencyType]); } catch (QueryException e) { curr = e.getMessage(); }
		report(quote + ' | ' + network + ' | ' + curr + ' ' + Schema.SObjectType.Opportunity.fields.getMap().containsKey('currencyisocode'));
	}
	@IsTest static void deleteByColdPrefix() {
		Database.DeleteResult one = Database.delete('810000000000001AAA', false);
		List<Database.DeleteResult> many = Database.delete(new List<Id>{'550000000000001AAA', '0Hn000000000001AAA'}, false);
		report(one.isSuccess() + ' ' + one.getErrors()[0].getStatusCode() + ' ' + many.size() + ' ' + many[1].getErrors()[0].getStatusCode());
	}
}`

func writeLazySpreadParityProject(t *testing.T, scratch string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	if scratch != "" {
		writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), scratch)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/LazySpreadParityTest.cls"), lazySpreadParityClass)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/LazySpreadParityTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	return root
}

func lazySpreadParityAnswers(t *testing.T, root string, noDisk bool) map[string]string {
	t.Helper()
	index := loadTestIndex(t, root)
	run := Run(index, Options{NoDiskCache: noDisk, Parallelism: 1, SelectedMethod: os.Getenv("LAZY_PARITY_METHOD")})
	out := make(map[string]string)
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			answer := string(c.Status)
			if c.Problem != nil {
				answer += ": " + c.Problem.Message
			}
			out[c.MethodName] = answer
		}
	}
	return out
}

func runLazySpreadParity(t *testing.T, eager bool, scratch string) map[string]string {
	t.Helper()
	prior := eagerStandardSpread.Load()
	eagerStandardSpread.Store(eager)
	defer eagerStandardSpread.Store(prior)
	return lazySpreadParityAnswers(t, writeLazySpreadParityProject(t, scratch), true)
}

// A lazy runtime written to the disk cache and restored from it answers what
// the eager spread answers. The lazy flag and the objects it holds persist; the
// shared source installs the rest.
func TestLazyStandardSpreadRestoredFromDiskMatchesEagerSpread(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}
	t.Setenv("LAZY_PARITY_METHOD", "soqlRelationshipsAndSubqueries")
	eager := runLazySpreadParity(t, true, "")
	enableDiskCacheForIsolationTest(t)
	prior := eagerStandardSpread.Load()
	eagerStandardSpread.Store(false)
	defer eagerStandardSpread.Store(prior)
	root := writeLazySpreadParityProject(t, "")
	InvalidateRuntimeCaches()
	built := lazySpreadParityAnswers(t, root, false)
	InvalidateRuntimeCaches()
	restored := lazySpreadParityAnswers(t, root, false)
	if len(eager) != 1 || !reflect.DeepEqual(eager, built) || !reflect.DeepEqual(eager, restored) {
		t.Fatalf("eager %v\nbuilt  %v\nrestored %v", eager, built, restored)
	}
}

// The describe-every-object method is split by position into this many chunks,
// each its own top-level test, so CI shard planning can place them separately.
const lazySpreadParityDescribeChunks = 4

var lazySpreadParityMethodPattern = regexp.MustCompile(`@IsTest static void (\w+)\(`)

func lazySpreadParityMethods(chunk bool) []string {
	var out []string
	for _, match := range lazySpreadParityMethodPattern.FindAllStringSubmatch(lazySpreadParityClass, -1) {
		if strings.HasPrefix(match[1], "describeEveryObject") == chunk {
			out = append(out, match[1])
		}
	}
	return out
}

func runLazySpreadParityMethod(t *testing.T, eager bool, scratch, method string) map[string]string {
	t.Helper()
	t.Setenv("LAZY_PARITY_METHOD", method)
	return runLazySpreadParity(t, eager, scratch)
}

func compareLazySpreadParity(t *testing.T, scratch string, methods []string) {
	t.Helper()
	if len(methods) == 0 {
		t.Fatal("no parity methods selected")
	}
	for _, method := range methods {
		eager := runLazySpreadParityMethod(t, true, scratch, method)
		lazy := runLazySpreadParityMethod(t, false, scratch, method)
		if len(eager) != 1 {
			t.Fatalf("%s: eager run reported %d cases: %v", method, len(eager), eager)
		}
		for name, answer := range eager {
			if !strings.HasPrefix(answer, string(testreport.StatusFail)) {
				t.Errorf("%s: eager answer is not a reported digest: %s", name, answer)
			}
			if answer != lazy[name] {
				t.Errorf("%s\n eager %s\n lazy  %s", name, answer, lazy[name])
			}
		}
		if len(lazy) != len(eager) {
			t.Errorf("%s: case counts differ: eager %d, lazy %d", method, len(eager), len(lazy))
		}
	}
}

const (
	lazySpreadParityNoInputs = ""
	lazySpreadParityFeatures = `{"edition":"Enterprise","features":["Communities","NetworksEnabled","Quotes","MultiCurrency","PersonAccounts"]}`
)

// The lazy spread must give the answers the eager spread gives. Each shape runs
// as several top-level tests: one per quarter of the describe-every-object
// method, and one for every other method.

func TestLazyStandardSpreadParityNoInputsDescribe0(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityNoInputs, lazySpreadParityMethods(true)[0:1])
}
func TestLazyStandardSpreadParityNoInputsDescribe1(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityNoInputs, lazySpreadParityMethods(true)[1:2])
}
func TestLazyStandardSpreadParityNoInputsDescribe2(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityNoInputs, lazySpreadParityMethods(true)[2:3])
}
func TestLazyStandardSpreadParityNoInputsDescribe3(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityNoInputs, lazySpreadParityMethods(true)[3:4])
}
func TestLazyStandardSpreadParityNoInputsRest(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityNoInputs, lazySpreadParityMethods(false))
}
func TestLazyStandardSpreadParityFeaturesDescribe0(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityFeatures, lazySpreadParityMethods(true)[0:1])
}
func TestLazyStandardSpreadParityFeaturesDescribe1(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityFeatures, lazySpreadParityMethods(true)[1:2])
}
func TestLazyStandardSpreadParityFeaturesDescribe2(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityFeatures, lazySpreadParityMethods(true)[2:3])
}
func TestLazyStandardSpreadParityFeaturesDescribe3(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityFeatures, lazySpreadParityMethods(true)[3:4])
}
func TestLazyStandardSpreadParityFeaturesRest(t *testing.T) {
	compareLazySpreadParity(t, lazySpreadParityFeatures, lazySpreadParityMethods(false))
}

// The chunk count and the methods above must agree.
func TestLazyStandardSpreadParityCoversEveryMethod(t *testing.T) {
	if got := len(lazySpreadParityMethods(true)); got != lazySpreadParityDescribeChunks {
		t.Fatalf("%d describe chunks in the class, tests cover %d", got, lazySpreadParityDescribeChunks)
	}
	if len(lazySpreadParityMethods(false)) == 0 {
		t.Fatal("no other parity methods")
	}
}

// A query whose child subquery names a cold object completes the lazy spread.
// That must cost about what the eager spread costs, not seconds per test.
func TestLazyStandardSpreadColdChildSubqueryCost(t *testing.T) {
	const class = `@IsTest
private class ColdChildTest {
	@IsTest static void a() { System.assertEquals(0, [SELECT Id, (SELECT Id FROM CampaignMembers) FROM Campaign].size()); }
	@IsTest static void b() { System.assertEquals(0, [SELECT Id, (SELECT Id FROM CampaignMembers) FROM Campaign].size()); }
	@IsTest static void c() { System.assertEquals(0, [SELECT Id, (SELECT Id FROM CampaignMembers) FROM Campaign].size()); }
}`
	wall := func(eager bool) time.Duration {
		prior := eagerStandardSpread.Load()
		eagerStandardSpread.Store(eager)
		defer eagerStandardSpread.Store(prior)
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ColdChildTest.cls"), class)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ColdChildTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		started := time.Now()
		run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
		elapsed := time.Since(started)
		if summary := run.Summary(); summary.Total != 3 || summary.Passed != 3 {
			t.Fatalf("eager=%v summary = %#v", eager, summary)
		}
		return elapsed
	}
	eager, lazy := wall(true), wall(false)
	t.Logf("cold child subquery, 3 tests: eager %v, lazy %v", eager, lazy)
	if lazy > 3*eager+2*time.Second {
		t.Fatalf("lazy spread took %v against %v eager", lazy, eager)
	}
}
