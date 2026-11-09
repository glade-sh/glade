public class LoadDataChunkProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        try {
            List<Account> rows = (List<Account>) Test.loadData(Account.SObjectType, 'LdChunkData');
            Integer withoutId = 0; Integer withTrigger = 0; Set<Id> ids = new Set<Id>();
            for (Account a : rows) {
                if (a.Id == null) { withoutId++; } else { ids.add(a.Id); }
                if (a.AccountNumber == 'trg') { withTrigger++; }
            }
            o.put('returned', rows.size());
            o.put('withoutId', withoutId);
            o.put('distinctIds', ids.size());
            o.put('returnedWithTriggerValue', withTrigger);
            o.put('stored', [SELECT COUNT() FROM Account WHERE Name LIKE 'LdChunk%']);
            o.put('storedWithTriggerValue', [SELECT COUNT() FROM Account WHERE Name LIKE 'LdChunk%' AND AccountNumber = 'trg']);
        } catch (Exception e) { o.put('error', e.getTypeName() + ':' + e.getMessage()); }
        // plain DML on the same shapes, for comparison with the loaded records
        Account one = new Account(Name = 'LdChunkSingle'); Database.insert(one, AccessLevel.SYSTEM_MODE);
        o.put('plainSingleVariable', one.AccountNumber);
        List<Account> many = new List<Account>();
        for (Integer i = 0; i < 250; i++) { many.add(new Account(Name = 'LdChunkPlain' + i)); }
        Database.insert(many, AccessLevel.SYSTEM_MODE);
        Integer reflected = 0; for (Account a : many) { if (a.AccountNumber == 'trg') { reflected++; } }
        o.put('plainBulkReflected', reflected);
        return o;
    }
}
