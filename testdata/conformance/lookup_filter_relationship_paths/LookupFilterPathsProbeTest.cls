@IsTest
private class LookupFilterPathsProbeTest {
    private static final String EXPECTED = 'cNotProviderContact|patient|OK;cNotProviderContact|provider|FIELD_FILTER_VALIDATION_EXCEPTION;cProviderContactOpt|patient|OK;cProviderContactOpt|provider|OK;cProviderContact|patient|FIELD_FILTER_VALIDATION_EXCEPTION;cProviderContact|provider|OK;personRecordTypeKept|patient|Patient|true;personRecordTypeKept|provider|Provider|true;tContactRtDev|patient|FIELD_FILTER_VALIDATION_EXCEPTION;tContactRtDev|provider|OK;tNotPerson|business|OK;tNotPerson|patient|FIELD_FILTER_VALIDATION_EXCEPTION;tNotPerson|provider|FIELD_FILTER_VALIDATION_EXCEPTION;tNotProviderRt|business|OK;tNotProviderRt|patient|OK;tNotProviderRt|provider|FIELD_FILTER_VALIDATION_EXCEPTION;tPersonOnly|business|FIELD_FILTER_VALIDATION_EXCEPTION;tPersonOnly|patient|OK;tPersonOnly|provider|OK;tProviderOpt|business|OK;tProviderOpt|patient|OK;tProviderOpt|provider|OK;tProviderRt|business|FIELD_FILTER_VALIDATION_EXCEPTION;tProviderRt|patient|FIELD_FILTER_VALIDATION_EXCEPTION;tProviderRt|provider|OK;tRtDev|business|FIELD_FILTER_VALIDATION_EXCEPTION;tRtDev|patient|FIELD_FILTER_VALIDATION_EXCEPTION;tRtDev|provider|OK;tRtNameNe|business|OK;tRtNameNe|patient|OK;tRtNameNe|provider|FIELD_FILTER_VALIDATION_EXCEPTION;tRtName|business|FIELD_FILTER_VALIDATION_EXCEPTION;tRtName|patient|FIELD_FILTER_VALIDATION_EXCEPTION;tRtName|provider|OK;uProviderRtChangeLookup|patient|FIELD_FILTER_VALIDATION_EXCEPTION;uProviderRtNameAfterTargetChanged|name|FIELD_FILTER_VALIDATION_EXCEPTION;uProviderRtResetLookup|samelookup|FIELD_FILTER_VALIDATION_EXCEPTION';
    private static Map<String, Id> rts = new Map<String, Id>();

    private static String outcome(Database.SaveResult r) {
        if (r.isSuccess()) { return 'OK'; }
        return String.valueOf(r.getErrors()[0].getStatusCode());
    }

    private static Account person(String rt, String last) {
        Account a = new Account(LastName = last, RecordTypeId = rts.get(rt));
        insert a;
        return a;
    }

    @IsTest
    static void observeLookupFilterPaths() {
        for (RecordType r : [SELECT Id, DeveloperName FROM RecordType WHERE SobjectType = 'Account']) { rts.put(r.DeveloperName, r.Id); }
        List<String> rows = new List<String>();
        Account prov = person('Provider', 'ProvA');
        Account pat = person('Patient', 'PatA');
        Account biz = new Account(Name = 'BizA', RecordTypeId = rts.get('Org_Standard'));
        insert biz;
        Account provRt = [SELECT RecordType.DeveloperName, IsPersonAccount, PersonContactId FROM Account WHERE Id = :prov.Id];
        Account patRt = [SELECT RecordType.DeveloperName, IsPersonAccount, PersonContactId FROM Account WHERE Id = :pat.Id];
        rows.add('personRecordTypeKept|provider|' + provRt.RecordType.DeveloperName + '|' + provRt.IsPersonAccount);
        rows.add('personRecordTypeKept|patient|' + patRt.RecordType.DeveloperName + '|' + patRt.IsPersonAccount);
        Map<String, Account> accts = new Map<String, Account>{'provider' => prov, 'patient' => pat, 'business' => biz};
        Map<String, String> fieldCase = new Map<String, String>{
            'PersonOnly__c' => 'tPersonOnly', 'NotPerson__c' => 'tNotPerson', 'ProviderRt__c' => 'tProviderRt',
            'NotProviderRt__c' => 'tNotProviderRt', 'ProviderOpt__c' => 'tProviderOpt'};
        for (String f : fieldCase.keySet()) {
            for (String k : new List<String>{'provider', 'patient', 'business'}) {
                Trail__c t = new Trail__c(Name = 't');
                t.put(f, accts.get(k).Id);
                rows.add(fieldCase.get(f) + '|' + k + '|' + outcome(Database.insert(t, false)));
            }
        }
        Map<String, String> fieldCase2 = new Map<String, String>{
            'RtDev__c' => 'tRtDev', 'RtName__c' => 'tRtName', 'RtNameNe__c' => 'tRtNameNe'};
        for (String f : fieldCase2.keySet()) {
            for (String k : new List<String>{'provider', 'patient', 'business'}) {
                Trail2__c t = new Trail2__c(Name = 't');
                t.put(f, accts.get(k).Id);
                rows.add(fieldCase2.get(f) + '|' + k + '|' + outcome(Database.insert(t, false)));
            }
        }
        for (String k : new List<String>{'provider', 'patient'}) {
            Account a = accts.get(k);
            Id cid = ((Account)[SELECT PersonContactId FROM Account WHERE Id = :a.Id]).PersonContactId;
            Trail2__c t = new Trail2__c(Name = 't', ContactRtDev__c = cid);
            rows.add('tContactRtDev|' + k + '|' + outcome(Database.insert(t, false)));
            for (String f : new List<String>{'ProviderContact__c', 'NotProviderContact__c', 'ProviderContactOpt__c'}) {
                Case c = new Case(Subject = 's');
                c.put(f, cid);
                rows.add('c' + f.replace('__c', '') + '|' + k + '|' + outcome(Database.insert(c, false)));
            }
        }
        // Updates: change the lookup to a non-matching account; change a different field after the target changed.
        Trail__c u = new Trail__c(Name = 'u', ProviderRt__c = prov.Id);
        insert u;
        u.ProviderRt__c = pat.Id;
        rows.add('uProviderRtChangeLookup|patient|' + outcome(Database.update(u, false)));
        Trail__c u2 = new Trail__c(Name = 'u2', ProviderRt__c = prov.Id);
        insert u2;
        Account provUpd = new Account(Id = prov.Id, RecordTypeId = rts.get('Patient'));
        update provUpd;
        u2.Name = 'u2b';
        rows.add('uProviderRtNameAfterTargetChanged|name|' + outcome(Database.update(u2, false)));
        u2.ProviderRt__c = prov.Id;
        rows.add('uProviderRtResetLookup|samelookup|' + outcome(Database.update(u2, false)));
        rows.sort();
        System.assertEquals(EXPECTED, String.join(rows, ';'));
    }
}
