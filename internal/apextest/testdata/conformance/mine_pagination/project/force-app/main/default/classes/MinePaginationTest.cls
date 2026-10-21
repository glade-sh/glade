@IsTest private without sharing class MinePaginationTest {
    private static List<String> names(List<SObject> records) {
        List<String> result = new List<String>();
        for (SObject record : records) result.add((String)record.get('Name'));
        return result;
    }

    private static Map<String, Object> snapshot(AccessLevel access, Boolean explicitSystemMode) {
        String pattern = 'MinePage%';
        Map<String, Object> binds = new Map<String, Object>{'pattern' => pattern};
        String query = 'SELECT Id, Name FROM Account USING SCOPE Mine WHERE Name LIKE :pattern'
            + (explicitSystemMode ? ' WITH SYSTEM_MODE' : '') + ' ORDER BY Name';
        Database.PaginationCursor cursor;
        try {
            cursor = Database.getPaginationCursorWithBinds(query, binds, access);
        } catch (QueryException error) {
            return new Map<String, Object>{'type' => error.getTypeName(), 'message' => error.getMessage()};
        }
        Integer count = cursor.getNumRecords();
        return new Map<String, Object>{
            'count' => count,
            'first' => count == 0 ? new List<String>() : names(cursor.fetchPage(0, 1).getRecords()),
            'offset' => count < 2 ? new List<String>() : names(cursor.fetchPage(1, 1).getRecords()),
            'ordinaryCount' => Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Name LIKE :pattern', binds, access)
        };
    }

    private static void observe(String method, AccessLevel access, Boolean explicitSystemMode) {
        Id actor = UserInfo.getUserId();
        User other = new User(Alias = 'minepage', LastName = 'Pagination Owner', Email = 'page@example.test',
            Username = 'page.' + UserInfo.getOrganizationId() + '.' + String.valueOf(Datetime.now().getTime()) + '@example.test',
            ProfileId = UserInfo.getProfileId(), TimeZoneSidKey = 'GMT', LocaleSidKey = 'en_US',
            EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        insert other;
        Map<String, Object> result = new Map<String, Object>();
        System.runAs(new User(Id = actor)) {
            insert new List<Account>{new Account(Name = 'MinePage A'), new Account(Name = 'MinePage B'),
                new Account(Name = 'MinePage C', OwnerId = other.Id)};
            result.put('actor', snapshot(access, explicitSystemMode));
            System.runAs(other) { result.put('other', snapshot(access, explicitSystemMode)); }
            result.put('restored', snapshot(access, explicitSystemMode));
        }
        System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(result));
    }

    @IsTest static void systemModeMine() {
        observe('systemModeMine', AccessLevel.SYSTEM_MODE, false);
    }

    @IsTest static void userModeMine() {
        observe('userModeMine', AccessLevel.USER_MODE, false);
    }

    @IsTest static void explicitSystemMine() {
        observe('explicitSystemMine', AccessLevel.SYSTEM_MODE, true);
    }
}
