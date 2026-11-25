public class ZephyrTextProbe {
    public static Map<String, Object> observed = new Map<String, Object>();

    public static void record(String stage, Zephyr__c row) {
        observed.put(stage + '-name', row.Name);
        observed.put(stage + '-phrase', row.Phrase__c);
        observed.put(stage + '-blank', row.Blank__c);
        observed.put(stage + '-trigger-write', row.TriggerWrite__c);
    }

    public static Map<String, Object> observe() {
        Zephyr__c row = new Zephyr__c(Name = '  Zephyr  ', Phrase__c = '\t  pale  blue \t',
            Blank__c = ' \t ');
        record('input', row);
        insert row;
        record('caller', row);
        record('stored', [SELECT Name, Phrase__c, Blank__c, TriggerWrite__c
            FROM Zephyr__c WHERE Id = :row.Id]);

        Account account = new Account(Name = '  Zephyr account  ', Site = '\t site \t');
        Database.SaveResult result = Database.insert(account, false);
        observed.put('account-success', result.isSuccess());
        return observed;
    }
}
