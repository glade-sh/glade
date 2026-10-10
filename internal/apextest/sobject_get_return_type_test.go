package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The temporal Object-null and String-null contracts are retained in
// testdata/conformance/corpus_rt_sobject_get.json. These owned regressions
// exercise the declared Object return of dynamic field access at the call site.
const sObjectGetReturnTypeSource = `@IsTest private class SObjectGetReturnTypeTest {
    @IsTest static void unsetDateByName() {
        Contact row = new Contact();
        System.assertEquals(null, Date.valueOf(row.get('Birthdate')));
    }
    @IsTest static void unsetDateByToken() {
        SObject row = new Contact();
        System.assertEquals(null, Date.valueOf(row.get(Contact.Birthdate)));
    }
    @IsTest static void explicitNullDateByName() {
        Contact row = new Contact(Birthdate=null);
        System.assertEquals(null, Date.valueOf(row.get('Birthdate')));
    }
    @IsTest static void explicitNullDateByToken() {
        Contact row = new Contact(Birthdate=null);
        System.assertEquals(null, Date.valueOf(row.get(Contact.Birthdate)));
    }
    @IsTest static void unsetDatetimeByName() {
        Event row = new Event();
        System.assertEquals(null, Datetime.valueOf(row.get('StartDateTime')));
    }
    @IsTest static void unsetDatetimeByToken() {
        SObject row = new Event();
        System.assertEquals(null, Datetime.valueOf(row.get(Event.StartDateTime)));
    }
    @IsTest static void explicitNullDatetimeByName() {
        Event row = new Event(); row.put('StartDateTime', null);
        System.assertEquals(null, Datetime.valueOf(row.get('StartDateTime')));
    }
    @IsTest static void explicitNullDatetimeByToken() {
        Event row = new Event(); row.put(Event.StartDateTime, null);
        System.assertEquals(null, Datetime.valueOf(row.get(Event.StartDateTime)));
    }
    @IsTest static void dateObjectNullControl() {
        Object value;
        System.assertEquals(null, Date.valueOf(value));
    }
    @IsTest static void datetimeObjectNullControl() {
        Object value;
        System.assertEquals(null, Datetime.valueOf(value));
    }
    @IsTest static void dateStringNullRejected() {
        String value;
        try { Date.valueOf(value); System.assert(false, 'String null must throw'); }
        catch (NullPointerException e) { System.assertEquals('Argument cannot be null.', e.getMessage()); }
    }
    @IsTest static void datetimeStringNullRejected() {
        String value;
        try { Datetime.valueOf(value); System.assert(false, 'String null must throw'); }
        catch (NullPointerException e) { System.assertEquals('Argument cannot be null.', e.getMessage()); }
    }
    @IsTest static void dateStringCastNullRejected() {
        Account row = new Account();
        try { Date.valueOf((String)row.get('Description')); System.assert(false, 'String cast null must throw'); }
        catch (NullPointerException e) { System.assertEquals('Argument cannot be null.', e.getMessage()); }
    }
    @IsTest static void datetimeStringCastNullRejected() {
        Account row = new Account();
        try { Datetime.valueOf((String)row.get(Account.Description)); System.assert(false, 'String cast null must throw'); }
        catch (NullPointerException e) { System.assertEquals('Argument cannot be null.', e.getMessage()); }
    }
    @IsTest static void populatedDate() {
        Date value = Date.newInstance(2026,5,4);
        Contact row = new Contact(Birthdate=value);
        System.assertEquals(value, Date.valueOf(row.get('Birthdate')));
        System.assertEquals(value, Date.valueOf(row.get(Contact.Birthdate)));
        System.assertEquals(value, row.Birthdate);
    }
    @IsTest static void populatedDatetime() {
        Datetime value = Datetime.newInstanceGmt(2026,5,2,1,2,3);
        Event row = new Event(); row.put('StartDateTime', value);
        System.assertEquals(value, Datetime.valueOf(row.get('StartDateTime')));
        System.assertEquals(value, Datetime.valueOf(row.get(Event.StartDateTime)));
        System.assertEquals(value, row.StartDateTime);
    }
    @IsTest static void dateObjectStringRejected() {
        Account row = new Account(Description='2026-05-04');
        try { Date.valueOf(row.get('Description')); System.assert(false, 'Object overload must reject String payload'); }
        catch (TypeException e) { System.assertEquals('Invalid date: 2026-05-04', e.getMessage()); }
        System.assertEquals(Date.newInstance(2026,5,4), Date.valueOf((String)row.get('Description')));
        System.assertEquals(Date.newInstance(2026,5,4), Date.valueOf(row.Description));
    }
    @IsTest static void datetimeObjectStringRejected() {
        Account row = new Account(Description='2026-05-02 01:02:03');
        try { Datetime.valueOf(row.get(Account.Description)); System.assert(false, 'Object overload must reject String payload'); }
        catch (TypeException e) { System.assertEquals('Invalid date/time: 2026-05-02 01:02:03', e.getMessage()); }
        System.assertEquals(Datetime.newInstance(2026,5,2,1,2,3), Datetime.valueOf((String)row.get(Account.Description)));
        System.assertEquals(Datetime.newInstance(2026,5,2,1,2,3), Datetime.valueOf(row.Description));
    }
}
`

func TestSObjectGetDeclaredObjectReturnType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "SObjectGetReturnTypeTest.cls")
			writeFile(t, path, sObjectGetReturnTypeSource)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 18 || summary.Passed != 18 {
				raw, _ := json.Marshal(run)
				t.Fatalf("dynamic field return type: %s", raw)
			}
		})
	}
}
