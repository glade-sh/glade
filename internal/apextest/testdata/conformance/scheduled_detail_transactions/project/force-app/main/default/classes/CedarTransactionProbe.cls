public class CedarTransactionProbe implements Schedulable, Database.Batchable<SObject> {
    public void execute(SchedulableContext context) {}
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account WHERE Name = \'cedar empty scope\'');
    }
    public void execute(Database.BatchableContext context, List<SObject> scope) {}
    public void finish(Database.BatchableContext context) {}
    public static String submit(Boolean batch, String name) {
        if (batch) return System.scheduleBatch(new CedarTransactionProbe(), name, 60);
        return System.schedule(name, '0 0 0 1 1 ? 2099', new CedarTransactionProbe());
    }
    public static Map<String,Object> observe(Boolean batch, String firstName, String secondName) {
        CronTrigger beforeRow = [SELECT Id, CronJobDetailId FROM CronTrigger WHERE CronJobDetail.Name = :firstName];
        String ticket = submit(batch, secondName);
        CronTrigger secondRow = [SELECT Id, CronJobDetailId, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :ticket];
        CronTrigger firstRow = [SELECT Id, CronJobDetailId, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :beforeRow.Id];
        Set<String> names = new Set<String>{firstName, secondName};
        return new Map<String,Object>{
            'freshTrigger' => firstRow.Id != secondRow.Id,
            'freshDetail' => firstRow.CronJobDetailId != secondRow.CronJobDetailId,
            'firstDetailPreserved' => beforeRow.CronJobDetailId == firstRow.CronJobDetailId,
            'firstNameMatches' => firstRow.CronJobDetail.Name == firstName,
            'secondNameMatches' => secondRow.CronJobDetail.Name == secondName,
            'firstJobType' => firstRow.CronJobDetail.JobType,
            'secondJobType' => secondRow.CronJobDetail.JobType,
            'triggerCount' => [SELECT COUNT() FROM CronTrigger WHERE CronJobDetail.Name IN :names],
            'detailCount' => [SELECT COUNT() FROM CronJobDetail WHERE Name IN :names]
        };
    }
}
