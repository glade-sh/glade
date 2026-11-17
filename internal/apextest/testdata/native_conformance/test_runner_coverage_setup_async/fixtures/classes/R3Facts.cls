public class R3Facts {
    public static String facts() {
        Map<String, Integer> counts = new Map<String, Integer>();
        for (Account a : [SELECT Name FROM Account WHERE Name LIKE 'R3-%' ORDER BY Name]) {
            Integer n = counts.get(a.Name);
            counts.put(a.Name, n == null ? 1 : n + 1);
        }
        Integer contacts = [SELECT count() FROM Contact WHERE LastName = 'R3-CDC'];
        Integer crons = [SELECT count() FROM CronTrigger];
        List<String> jobs = new List<String>();
        for (AsyncApexJob j : [SELECT JobType, Status, ApexClass.Name, NumberOfErrors FROM AsyncApexJob ORDER BY ApexClass.Name, JobType]) {
            jobs.add((j.ApexClass == null ? 'none' : j.ApexClass.Name) + '/' + j.JobType + '/' + j.Status + '/' + j.NumberOfErrors);
        }
        return 'FACTS markers=' + counts + ' cdcContacts=' + contacts + ' cronTriggers=' + crons
            + ' queueableJobs=' + Limits.getQueueableJobs() + ' futureCalls=' + Limits.getFutureCalls()
            + ' asyncCalls=' + Limits.getAsyncCalls() + ' jobs=' + jobs;
    }
}
