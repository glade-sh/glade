@IsTest
private class CursorMapObservationTest {
    @IsTest static void tiedCursor() {
        List<Account> records = seed(3);
        Datetime stamp = Datetime.newInstanceGmt(2026, 3, 2, 11, 12, 13);
        for (Account record : records) Test.setCreatedDate(record.Id, stamp);
        List<Account> first = [SELECT Id, CreatedDate FROM Account ORDER BY CreatedDate DESC, Id DESC LIMIT 1];
        Datetime cursorStamp = Datetime.newInstance(first[0].CreatedDate.getTime());
        Id cursorId = first[0].Id;
        Map<String,Object> binds = new Map<String,Object>{'stamp'=>cursorStamp, 'key'=>cursorId};
        List<Account> next = Database.queryWithBinds(
            'SELECT Id FROM Account WHERE (CreatedDate < :stamp OR (CreatedDate = :stamp AND Id < :key)) ORDER BY CreatedDate DESC, Id DESC',
            binds, AccessLevel.SYSTEM_MODE
        );
        Set<Id> remaining = new Map<Id,Account>(next).keySet();
        emit('tiedCursor', new Map<String,Object>{'remaining'=>next.size(), 'repeatsCursor'=>remaining.contains(cursorId), 'equalStamp'=>[SELECT Count() FROM Account WHERE CreatedDate = :cursorStamp]});
    }

    @IsTest static void distinctCursor() {
        List<Account> records = seed(3);
        Datetime stamp = Datetime.newInstanceGmt(2026, 3, 2, 11, 12, 13);
        for (Integer i=0; i<records.size(); i++) Test.setCreatedDate(records[i].Id, stamp.addMinutes(-i));
        List<Account> first = [SELECT Id, CreatedDate FROM Account ORDER BY CreatedDate DESC, Id DESC LIMIT 1];
        Datetime cursorStamp = Datetime.newInstance(first[0].CreatedDate.getTime());
        Id cursorId = first[0].Id;
        List<Account> next = Database.queryWithBinds(
            'SELECT Id FROM Account WHERE (CreatedDate < :stamp OR (CreatedDate = :stamp AND Id < :key)) ORDER BY CreatedDate DESC, Id DESC',
            new Map<String,Object>{'stamp'=>cursorStamp,'key'=>cursorId}, AccessLevel.SYSTEM_MODE
        );
        emit('distinctCursor', new Map<String,Object>{'remaining'=>next.size(), 'repeatsCursor'=>new Map<Id,Account>(next).containsKey(cursorId)});
    }

    @IsTest static void millisecondCursor() {
        Account record = seed(1)[0];
        Datetime stamp = Datetime.newInstance(Datetime.newInstanceGmt(2026, 3, 2, 11, 12, 13).getTime()+321L);
        Test.setCreatedDate(record.Id, stamp);
        Account stored = [SELECT Id, CreatedDate FROM Account WHERE Id=:record.Id];
        Map<String,Object> binds = new Map<String,Object>{'stamp'=>stamp};
        List<Account> equalRows = Database.queryWithBinds('SELECT Id FROM Account WHERE CreatedDate = :stamp', binds, AccessLevel.SYSTEM_MODE);
        List<Account> earlierRows = Database.queryWithBinds('SELECT Id FROM Account WHERE CreatedDate < :stamp', binds, AccessLevel.SYSTEM_MODE);
        List<Account> laterRows = Database.queryWithBinds('SELECT Id FROM Account WHERE CreatedDate > :stamp', binds, AccessLevel.SYSTEM_MODE);
        emit('millisecondCursor', new Map<String,Object>{'storedMillis'=>stored.CreatedDate.getTime()-Datetime.newInstanceGmt(2026,3,2,11,12,13).getTime(), 'equal'=>equalRows.size(), 'earlier'=>earlierRows.size(), 'later'=>laterRows.size()});
    }

    @IsTest static void nullMapCopy() {
        Map<String,Object> source = (Map<String,Object>)JSON.deserializeUntyped('null');
        Map<String,Object> observation = new Map<String,Object>{'sourceNull'=>source==null, 'caught'=>false};
        try {
            Map<String,Object> copied = new Map<String,Object>(source);
            observation.put('copyNull', copied==null);
        } catch (Exception failure) {
            observation.put('caught', true);
            observation.put('type', failure.getTypeName());
            observation.put('message', failure.getMessage());
        }
        emit('nullMapCopy', observation);
    }

    @IsTest static void populatedMapCopy() {
        Map<String,Object> source = new Map<String,Object>{'alpha'=>1};
        Map<String,Object> copied = new Map<String,Object>(source);
        copied.put('beta',2);
        emit('populatedMapCopy', new Map<String,Object>{'sourceSize'=>source.size(), 'copySize'=>copied.size(), 'retained'=>copied.get('alpha'), 'sourceHasBeta'=>source.containsKey('beta')});
    }

    private static List<Account> seed(Integer count) {
        List<Account> records = new List<Account>();
        for (Integer i=0; i<count; i++) records.add(new Account(Name='Cursor '+i));
        insert records;
        return records;
    }
    private static void emit(String method, Map<String,Object> observation) {
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }
}
