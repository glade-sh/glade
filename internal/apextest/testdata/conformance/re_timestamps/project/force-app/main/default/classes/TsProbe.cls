@IsTest
private class TsProbe {
    private static String row(Account q) {
        return q.CreatedDate.getTime() + ':' + q.LastModifiedDate.getTime() + ':' + q.SystemModstamp.getTime() + ':' + Math.mod(q.CreatedDate.getTime(),1000) + ':' + Math.mod(q.LastModifiedDate.getTime(),1000) + ':' + Math.mod(q.SystemModstamp.getTime(),1000);
    }
    @IsTest static void oneBatch() {
        List<Account> rows = new List<Account>{new Account(Name='T0'), new Account(Name='T1'), new Account(Name='T2')};
        insert rows;
        List<Account> q = [SELECT Id, Name, CreatedDate, LastModifiedDate, SystemModstamp FROM Account ORDER BY Name];
        List<String> parts = new List<String>();
        for (Account a : q) parts.add(a.Name + ':' + a.CreatedDate.getTime() + ':' + a.LastModifiedDate.getTime() + ':' + a.SystemModstamp.getTime());
        System.assert(false, 'T4|TEST1|' + String.join(parts, ';'));
    }
    @IsTest static void setCreatedDate() {
        Account a = new Account(Name='TS');
        insert a;
        Test.setCreatedDate(a.Id, Datetime.newInstance(1577934245678L));
        Account q1 = [SELECT CreatedDate, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
        Test.setCreatedDate(a.Id, Datetime.newInstance(1623053350321L));
        Account q2 = [SELECT CreatedDate, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
        a.Description = 'after set';
        update a;
        Account q3 = [SELECT CreatedDate, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
        System.assert(false, 'T4|TEST2|' + row(q1) + ';' + row(q2) + ';' + row(q3));
    }
    @IsTest static void twoStatements() {
        Account a = new Account(Name='TSA');
        insert a;
        Account b = new Account(Name='TSB');
        insert b;
        Test.setCreatedDate(a.Id, Datetime.newInstance(1551675967089L));
        Test.setCreatedDate(b.Id, Datetime.newInstance(1551675967089L));
        List<Account> q = [SELECT Name, CreatedDate, LastModifiedDate, SystemModstamp FROM Account ORDER BY Name];
        List<String> parts = new List<String>();
        for (Account x : q) parts.add(x.Name + ':' + x.CreatedDate.getTime() + ':' + x.LastModifiedDate.getTime() + ':' + x.SystemModstamp.getTime());
        System.assert(false, 'T4|TEST5|' + String.join(parts, ';'));
    }
    @IsTest static void updateSameTransaction() {
        Account a = new Account(Name='TSU');
        insert a;
        Account before = [SELECT CreatedDate, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
        a.Description = 'u';
        update a;
        Account afterQ = [SELECT CreatedDate, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
        System.assert(false, 'T4|TEST6|' + before.CreatedDate.getTime() + '|' + before.LastModifiedDate.getTime() + '|' + before.SystemModstamp.getTime() + '|' + afterQ.CreatedDate.getTime() + '|' + afterQ.LastModifiedDate.getTime() + '|' + afterQ.SystemModstamp.getTime());
    }
}
