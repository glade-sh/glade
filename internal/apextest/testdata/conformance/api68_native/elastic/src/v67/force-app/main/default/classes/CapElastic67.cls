public class CapElastic67 implements Database.Batchable<Integer> {
    public Iterable<Integer> start(Database.BatchableContext bc) {
        return new List<Integer>{1, 2};
    }
    public void execute(Database.BatchableContext bc, List<Integer> scope) {
        System.debug(LoggingLevel.ERROR, 'BATCH_SCOPE=' + scope.size());
        insert new Capture_Row__c(Case__c = 'elastic', Stage__c = 'scope',
            Body__c = 'BATCH_SCOPE=' + scope.size());
    }
    public void finish(Database.BatchableContext bc) {
        logLimits('FINISH');
        insert new Capture_Row__c(Case__c = 'elastic', Stage__c = 'finish',
            Body__c = 'BATCH_FINISHED=true');
    }
    public static void logLimits(String tag) {
        Map<String, System.OrgLimit> m = OrgLimits.getMap();
        for (String k : new List<String>{'DailyAsyncApexExecutions',
                 'DailyAsyncApexElasticExecutions'}) {
            System.debug(LoggingLevel.ERROR, tag + ';' + k + '=' +
                (m.containsKey(k) ? String.valueOf(m.get(k).getValue()) + '/' +
                 String.valueOf(m.get(k).getLimit()) : 'ABSENT'));
        }
    }
    public static void run() {
        logLimits('BEFORE');
        Id jobId = Database.executeBatch(new CapElastic67(), 1);
        System.debug(LoggingLevel.ERROR, 'JOB=' + jobId);
    }
    public static void readJobs() {
        for (AsyncApexJob j : [SELECT Id, Status, JobType, JobItemsProcessed, TotalJobItems
                               FROM AsyncApexJob WHERE JobType = 'BatchApex'
                               ORDER BY CreatedDate DESC LIMIT 3]) {
            System.debug(LoggingLevel.ERROR, 'JOBROW=' + j.Status + ';' + j.JobItemsProcessed
                + '/' + j.TotalJobItems);
        }
        for (Capture_Row__c r : [SELECT Stage__c, Body__c FROM Capture_Row__c
                                 WHERE Case__c = 'elastic' ORDER BY CreatedDate]) {
            System.debug(LoggingLevel.ERROR, 'ROW|' + r.Stage__c + '|' + r.Body__c);
        }
    }
}
