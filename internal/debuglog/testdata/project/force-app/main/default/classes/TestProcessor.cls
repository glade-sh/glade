public with sharing class TestProcessor {
    public static void run() {
        System.debug(LoggingLevel.INFO, 'start work');
        Account a = new Account(Name = 'Acme');
        insert a;
        List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Acme'];
        System.debug('rows=' + rows.size());
    }

    public static void fail() {
        throw new AuraHandledException('Nope');
    }
}
