@IsTest(SeeAllData=false)
private without sharing class DootSoqlU7SecondaryTestC1031 {
    @IsTest static void baseline() {
        System.assertEquals(2, 1 + 1, 'BASELINE|arithmetic');
    }

    @TestSetup static void setupUser() {
        insert as system new User(
            Alias='du7sec', LastName='DootU7SecondaryC1031',
            Email='capture@example.invalid', EmailEncodingKey='UTF-8',
            LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='GMT',
            ProfileId=UserInfo.getProfileId(),
            Username='DootU7SecondaryC1031.' + UserInfo.getOrganizationId() + '.'
                + System.now().getTime() + '@example.invalid'
        );
    }

    private static Map<String,Object> failure(String stage, Exception e) {
        Map<String,Object> x = new Map<String,Object>{
            'stage'=>stage, 'type'=>e.getTypeName(), 'message'=>e.getMessage()
        };
        if (e instanceof QueryException) {
            x.put('inaccessible', ((QueryException)e).getInaccessibleFields());
        }
        if (e instanceof DmlException) {
            DmlException d = (DmlException)e;
            List<Object> rows = new List<Object>();
            for (Integer i=0; i<d.getNumDml(); i++) {
                rows.add(new Map<String,Object>{'i'=>d.getDmlIndex(i),
                    'code'=>String.valueOf(d.getDmlStatusCode(i)),
                    'message'=>d.getDmlMessage(i), 'fields'=>d.getDmlFieldNames(i)});
            }
            x.put('dml', rows);
        }
        return x;
    }

    // Exact persisted values in seed order; no case-insensitive String == gate.
    private static List<Object> readback(List<Account> expected, Set<Id> ids) {
        Map<Id,Account> actual = new Map<Id,Account>([
            SELECT Id,Name,AccountNumber FROM Account WHERE Id IN :ids WITH SYSTEM_MODE
        ]);
        System.assertEquals(2, actual.size(), 'fixture exact two-row membership');
        List<Object> values = new List<Object>();
        for (Account e : expected) {
            System.assert(actual.containsKey(e.Id), 'fixture exact Id');
            Account a = actual.get(e.Id);
            System.assert(e.Name.equals(a.Name), 'fixture exact Name');
            System.assert(e.AccountNumber.equals(a.AccountNumber), 'fixture exact AccountNumber');
            values.add(new List<String>{String.valueOf(a.Id),a.Name,a.AccountNumber});
        }
        return values;
    }

    private static List<String> orderedIds(List<Account> rows, Set<Id> expected) {
        List<String> ordered = new List<String>();
        Set<Id> actual = new Set<Id>();
        for (Account row : rows) {
            System.assert(expected.contains(row.Id), 'query foreign Id');
            System.assert(!actual.contains(row.Id), 'query duplicate Id');
            actual.add(row.Id);
            ordered.add(String.valueOf(row.Id));
        }
        System.assertEquals(2, actual.size(), 'query exact two-row membership');
        return ordered;
    }

    private static void emit(Map<String,Object> o) {
        String j = JSON.serialize(o);
        if (j.length()>620) {
            j=JSON.serialize(new Map<String,Object>{
                'status'=>'inconclusive_overlength', 'originalLength'=>j.length()
            });
        }
        System.assert(false, 'CAPTURE|U7.secondaryKey|' + j);
    }

    @IsTest static void secondaryKey() {
        Map<String,Object> o = new Map<String,Object>();
        String stage='user-fixture';
        try {
            List<User> users = [SELECT Id,LocaleSidKey,TimeZoneSidKey FROM User
                WHERE LastName='DootU7SecondaryC1031' WITH SYSTEM_MODE];
            System.assertEquals(1, users.size(), 'fixture unique test user');
            System.assertEquals('en_US', users[0].LocaleSidKey, 'fixture saved locale');
            System.assertEquals('GMT', users[0].TimeZoneSidKey, 'fixture saved timezone');
            System.runAs(users[0]) {
                stage='runAs-gate';
                o.put('locale',UserInfo.getLocale());
                o.put('tz',UserInfo.getTimeZone().getID());
                o.put('userId',String.valueOf(UserInfo.getUserId()));
                System.assertEquals(users[0].Id,UserInfo.getUserId(),'fixture actual runAs user');
                System.assertEquals('en_US',UserInfo.getLocale(),'fixture actual en_US');
                System.assertEquals('GMT',UserInfo.getTimeZone().getID(),'fixture actual GMT');
                stage='mixed-fixture';
                // Exact distinct original U7 secondary-key pair, not the punctuation pair.
                List<Account> seed = new List<Account>{
                    new Account(Name='A',AccountNumber='z'),
                    new Account(Name='a',AccountNumber='a')
                };
                insert as system seed;
                Set<Id> ids = new Map<Id,Account>(seed).keySet();
                o.put('before',readback(seed,ids));
                stage='mixed-query';
                o.put('aa',orderedIds([SELECT Id FROM Account WHERE Id IN :ids
                    WITH SYSTEM_MODE ORDER BY Name ASC,AccountNumber ASC,Id ASC],ids));
                o.put('ad',orderedIds([SELECT Id FROM Account WHERE Id IN :ids
                    WITH SYSTEM_MODE ORDER BY Name ASC,AccountNumber DESC,Id ASC],ids));
                o.put('da',orderedIds([SELECT Id FROM Account WHERE Id IN :ids
                    WITH SYSTEM_MODE ORDER BY Name DESC,AccountNumber ASC,Id ASC],ids));
                stage='equal-primary-fixture';
                // Keep IDs/secondary values fixed; remove only the primary case distinction.
                update as system new Account(Id=seed[1].Id,Name='A');
                seed[1].Name='A';
                o.put('after',readback(seed,ids));
                stage='query-controls';
                o.put('ca',orderedIds([SELECT Id FROM Account WHERE Id IN :ids
                    WITH SYSTEM_MODE ORDER BY Name ASC,AccountNumber ASC,Id ASC],ids));
                o.put('cd',orderedIds([SELECT Id FROM Account WHERE Id IN :ids
                    WITH SYSTEM_MODE ORDER BY Name ASC,AccountNumber DESC,Id ASC],ids));
                o.put('stage',stage);
                o.put('status','observed');
            }
        } catch (Exception e) {
            o.put('status','inconclusive_exception');
            o.put('exception',failure(stage,e));
        }
        emit(o);
    }
}
