public class PerfRisk implements Database.Batchable<SObject>, Queueable {
    @AuraEnabled
    public static List<Account> uncachedAccounts(List<Id> ids) {
        List<Account> out = new List<Account>();
        for (Id accountId : ids) {
            out.add([SELECT Id, Name, Description FROM Account WHERE Id = :accountId]);
        }
        Schema.getGlobalDescribe();
        Schema.getGlobalDescribe();
        return out;
    }

    @InvocableMethod
    public static void invocableWork(List<Id> ids) {
        for (Id accountId : ids) {
            update new Account(Id = accountId, Description = 'changed');
        }
    }

    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id, Name FROM Account');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
        for (SObject row : scope) {
            System.enqueueJob(new PerfRisk());
        }
    }

    public void finish(Database.BatchableContext context) {
    }

    public void execute(QueueableContext context) {
    }

    public PageReference save() {
        List<Account> accounts = [SELECT Id, Name FROM Account];
        update accounts;
        return null;
    }
}
