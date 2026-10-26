public class WillowDetailProbe implements Schedulable, Database.Batchable<SObject> {
    public void execute(SchedulableContext context) {}
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account WHERE Name = \'willow detail empty scope\'');
    }
    public void execute(Database.BatchableContext context, List<SObject> scope) {}
    public void finish(Database.BatchableContext context) {}
    private static String enqueue(Boolean batch, String name) {
        if (batch) return System.scheduleBatch(new WillowDetailProbe(), name, 60);
        return System.schedule(name, '0 0 0 1 1 ? 2099', new WillowDetailProbe());
    }
    private static void snapshot(Map<String,Object> rows, String phase, String name, Id detail) {
        Set<String> names = new Set<String>{name};
        rows.put(phase + 'Triggers', [SELECT COUNT() FROM CronTrigger WHERE CronJobDetail.Name IN :names]);
        rows.put(phase + 'Details', [SELECT COUNT() FROM CronJobDetail WHERE JobType = '7' AND Name LIKE :name]);
        rows.put(phase + 'DetailById', [SELECT COUNT() FROM CronJobDetail WHERE Id = :detail]);
    }
    public static Map<String,Object> observe(Boolean batch, Boolean recorded) {
        String name = 'Willow detail ' + (batch ? 'batch' : 'schedule') + (recorded ? ' recorded' : ' queued');
        String keeperName = name + ' keeper';
        Map<String,Object> rows = new Map<String,Object>();
        if (recorded) Test.startTest();
        String keeper = enqueue(batch, keeperName);
        String first = enqueue(batch, name);
        if (recorded) Test.stopTest();
        CronTrigger original = [SELECT Id, CronJobDetailId, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :first];
        Id detail = original.CronJobDetailId;
        rows.put('nameMatches', original.CronJobDetail.Name == name);
        rows.put('jobType', original.CronJobDetail.JobType);
        snapshot(rows, 'before', name, detail);
        System.abortJob(first);
        snapshot(rows, 'aborted', name, detail);
        rows.put('keeperTriggers', [SELECT COUNT() FROM CronTrigger WHERE Id = :keeper]);
        rows.put('keeperDetails', [SELECT COUNT() FROM CronJobDetail WHERE Name = :keeperName]);
        String second = enqueue(batch, name);
        CronTrigger replacement = [SELECT Id, CronJobDetailId, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :second];
        rows.put('freshTrigger', first != second);
        rows.put('freshDetail', detail != replacement.CronJobDetailId);
        rows.put('replacementNameMatches', replacement.CronJobDetail.Name == name);
        rows.put('replacementJobType', replacement.CronJobDetail.JobType);
        snapshot(rows, 'rescheduled', name, replacement.CronJobDetailId);
        rows.put('oldDetailAfterReuse', [SELECT COUNT() FROM CronJobDetail WHERE Id = :detail]);
        System.abortJob(second);
        snapshot(rows, 'cleaned', name, replacement.CronJobDetailId);
        System.abortJob(keeper);
        rows.put('keeperCleaned', [SELECT COUNT() FROM CronJobDetail WHERE Name = :keeperName]);
        return rows;
    }
}
