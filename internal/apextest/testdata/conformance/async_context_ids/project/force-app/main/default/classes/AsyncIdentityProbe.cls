public class AsyncIdentityProbe {
    public static Map<String, Object> observe(String queueId, String batchId, String scheduleId) {
        Map<String, Object> result = new Map<String, Object>();
        for (Account row : [SELECT Name, Description, Phone FROM Account WHERE Name IN ('queueable','finalizer','batchStart','batchExecute','batchFinish','scheduled') ORDER BY Name]) {
            String expected = row.Name == 'scheduled' ? scheduleId : (row.Name.startsWith('batch') ? batchId : queueId);
            result.put(row.Name, new List<Object>{JSON.deserializeUntyped(row.Description), row.Phone.equals(expected), expected.length()});
        }
        return result;
    }
}
