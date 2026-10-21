public without sharing class OwnedScopeObservation {
    static List<String> labels(List<SObject> rows, Map<Id,String> names) {
        List<String> result=new List<String>();
        for(SObject row:rows) result.add(names.get((Id)row.get('Id')));
        result.sort(); return result;
    }
    static Map<String,Object> userRows(Set<Id> ids,Map<Id,String> names) {
        Map<String,Object> result=new Map<String,Object>();
        result.put('mine',labels(Database.query('SELECT Id FROM User USING SCOPE Mine WHERE Id IN :ids'),names));
        result.put('mineCount',Database.countQuery('SELECT COUNT() FROM User USING SCOPE Mine WHERE Id IN :ids'));
        result.put('everything',labels(Database.query('SELECT Id FROM User USING SCOPE Everything WHERE Id IN :ids'),names));
        result.put('ordinary',labels(Database.query('SELECT Id FROM User WHERE Id IN :ids'),names));
        return result;
    }
    static Map<String,Object> accountRows(Set<Id> ids,Map<Id,String> names) {
        Map<String,Object> result=new Map<String,Object>();
        result.put('mine',labels(Database.query('SELECT Id FROM Account USING SCOPE Mine WHERE Id IN :ids'),names));
        result.put('mineCount',Database.countQuery('SELECT COUNT() FROM Account USING SCOPE Mine WHERE Id IN :ids'));
        result.put('everything',labels(Database.query('SELECT Id FROM Account USING SCOPE Everything WHERE Id IN :ids'),names));
        result.put('ordinary',labels(Database.query('SELECT Id FROM Account WHERE Id IN :ids'),names));
        result.put('window',labels(Database.query('SELECT Id FROM Account USING SCOPE mInE WHERE Id IN :ids ORDER BY Name LIMIT 1 OFFSET 1'),names));
        result.put('whereCount',Database.countQuery('SELECT COUNT() FROM Account USING SCOPE Mine WHERE Id IN :ids AND Name = \'Owned scope B\''));
        return result;
    }
    public static Map<String,Object> observe(Boolean records) {
        Id actor=UserInfo.getUserId();
        Id profileId=[SELECT ProfileId FROM User WHERE Id=:actor].ProfileId;
        String nonce=UserInfo.getOrganizationId()+String.valueOf(Datetime.now().getTime());
        List<User> users=new List<User>();
        for(Integer n=0;n<2;n++) users.add(new User(IsActive=records,Alias='scope'+n,LastName='Owned scope '+n,Email='scope'+n+'@example.invalid',Username='owned.scope.'+n+'.'+nonce+'@example.invalid',ProfileId=profileId,TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'));
        insert users;
        Map<Id,String> userNames=new Map<Id,String>{actor=>'actor',users[0].Id=>'first',users[1].Id=>'second'};
        Set<Id> userIds=userNames.keySet();
        Map<String,Object> result=new Map<String,Object>{'usersActor'=>userRows(userIds,userNames)};
        if(records) {
            System.runAs(new User(Id=actor)) {
                List<Account> accounts=new List<Account>{new Account(Name='Owned scope A'),new Account(Name='Owned scope B'),new Account(Name='Owned scope C',OwnerId=users[0].Id)};
                insert accounts;
                Map<Id,String> names=new Map<Id,String>{accounts[0].Id=>'a',accounts[1].Id=>'b',accounts[2].Id=>'c'};
                Set<Id> ids=names.keySet();
                result.put('accountsActor',accountRows(ids,names));
                System.runAs(users[0]) {
                    result.put('usersFirst',userRows(userIds,userNames));
                    result.put('accountsFirst',accountRows(ids,names));
                }
                System.runAs(users[1]) { result.put('accountsSecond',accountRows(ids,names)); }
                result.put('accountsRestored',accountRows(ids,names));
            }
        }
        return result;
    }
}
