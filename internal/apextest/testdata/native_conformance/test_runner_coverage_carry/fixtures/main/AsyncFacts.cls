public class AsyncFacts {
    public static String facts() {
        Map<String, Integer> counts = new Map<String, Integer>();
        for (Account a : [SELECT Name FROM Account WHERE Name LIKE 'MK-%' ORDER BY Name]) {
            Integer n = counts.get(a.Name);
            counts.put(a.Name, n == null ? 1 : n + 1);
        }
        List<String> jobs = new List<String>();
        for (AsyncApexJob j : [SELECT JobType, Status, ApexClass.Name, NumberOfErrors FROM AsyncApexJob ORDER BY ApexClass.Name, JobType]) {
            jobs.add((j.ApexClass == null ? 'none' : j.ApexClass.Name) + '/' + j.JobType + '/' + j.Status + '/' + j.NumberOfErrors);
        }
        return 'FACTS markers=' + counts + ' queueableJobs=' + Limits.getQueueableJobs()
            + ' futureCalls=' + Limits.getFutureCalls() + ' asyncCalls=' + Limits.getAsyncCalls()
            + ' jobs=' + jobs;
    }
}
