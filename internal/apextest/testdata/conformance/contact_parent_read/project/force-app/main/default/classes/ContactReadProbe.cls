public with sharing class ContactReadProbe {
    public static Map<String, Object> observe() {
        if (!Test.isRunningTest()) {
            seed();
            return readRows();
        }
        User actor = new User(
            Alias='reader', Email='reader@example.invalid',
            Username='reader.' + UserInfo.getOrganizationId() + '@example.invalid',
            ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,
            TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US',
            EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US', LastName='Reader');
        insert actor;
        System.runAs(new User(Id=UserInfo.getUserId())) { seed(); }
        Map<String, Object> result;
        System.runAs(actor) { result = readRows(); }
        return result;
    }
    private static void seed() {
        delete [SELECT Id FROM Contact WHERE Account.Name='Meadow'];
        delete [SELECT Id FROM Account WHERE Name='Meadow'];
        Account parent = new Account(Name='Meadow');
        insert parent;
        insert new List<Contact>{
            new Contact(LastName='Amber', AccountId=parent.Id),
            new Contact(LastName='Cedar', AccountId=parent.Id)
        };
    }
    private static Map<String, Object> readRows() {
        String query = 'SELECT Id,LastName FROM Contact WHERE Account.Name = \'Meadow\' ORDER BY LastName';
        List<SObject> plain = Database.query(query);
        List<SObject> systemRows = Database.query(query, AccessLevel.SYSTEM_MODE);
        List<SObject> bound = Database.queryWithBinds(
            'SELECT Id FROM Contact WHERE Account.Name = :label',
            new Map<String,Object>{'label'=>'Meadow'}, AccessLevel.SYSTEM_MODE);
        List<AggregateResult> grouped = [SELECT AccountId, COUNT(Id) total FROM Contact WHERE Account.Name='Meadow' GROUP BY AccountId];
        Integer children = 0;
        for (Account parent : [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Name='Meadow']) {
            children += parent.Contacts.size();
        }
        return new Map<String,Object>{
            'plain'=>plain.size(), 'system'=>systemRows.size(), 'bound'=>bound.size(),
            'count'=>Database.countQuery('SELECT COUNT() FROM Contact WHERE Account.Name = \'Meadow\'', AccessLevel.SYSTEM_MODE),
            'boundCount'=>Database.countQueryWithBinds('SELECT COUNT() FROM Contact WHERE Account.Name = :label', new Map<String,Object>{'label'=>'Meadow'}, AccessLevel.SYSTEM_MODE),
            'groups'=>grouped.size(), 'groupCount'=>grouped.isEmpty() ? 0 : grouped[0].get('total'),
            'children'=>children
        };
    }
}
