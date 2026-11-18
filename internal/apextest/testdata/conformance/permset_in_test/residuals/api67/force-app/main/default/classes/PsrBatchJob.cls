public class PsrBatchJob implements Database.Batchable<SObject> {
    public static Integer runs = 0;
    public static Boolean sawPerm;

    public Database.QueryLocator start(Database.BatchableContext bc) {
        return Database.getQueryLocator([SELECT Id FROM PsrObj__c LIMIT 1]);
    }

    public void execute(Database.BatchableContext bc, List<SObject> scope) {
        runs++;
        sawPerm = FeatureManagement.checkPermission('PsrPerm');
    }

    public void finish(Database.BatchableContext bc) {
    }
}
