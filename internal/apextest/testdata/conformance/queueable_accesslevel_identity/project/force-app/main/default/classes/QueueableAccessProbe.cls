public class QueueableAccessProbe {
    public static Map<String,Object> seen = new Map<String,Object>();
    public class Job implements Queueable {
        public AccessLevel carried = System.AccessLevel.SYSTEM_MODE;
        public AccessLevel carriedUser = System.AccessLevel.USER_MODE;
        public List<AccessLevel> carriedList = new List<AccessLevel>{ System.AccessLevel.SYSTEM_MODE };
        public Map<String,AccessLevel> carriedMap = new Map<String,AccessLevel>{ 'k' => System.AccessLevel.USER_MODE };
        public Job() {
            QueueableAccessProbe.seen.put('beforeEnqueue', carried == System.AccessLevel.SYSTEM_MODE);
        }
        public void execute(QueueableContext context) {
            QueueableAccessProbe.seen.put('systemIdentity', carried == System.AccessLevel.SYSTEM_MODE);
            QueueableAccessProbe.seen.put('systemNotUser', carried == System.AccessLevel.USER_MODE);
            QueueableAccessProbe.seen.put('userIdentity', carriedUser == System.AccessLevel.USER_MODE);
            QueueableAccessProbe.seen.put('systemEquals', carried.equals(System.AccessLevel.SYSTEM_MODE));
            QueueableAccessProbe.seen.put('listIdentity', carriedList[0] == System.AccessLevel.SYSTEM_MODE);
            QueueableAccessProbe.seen.put('mapIdentity', carriedMap.get('k') == System.AccessLevel.USER_MODE);
            QueueableAccessProbe.seen.put('text', String.valueOf(carried));
            QueueableAccessProbe.seen.put('notNull', carried != null);
        }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        if (!Test.isRunningTest()) { result.put('context', 'anonymous'); return result; }
        Test.startTest();
        System.enqueueJob(new Job());
        Test.stopTest();
        result.putAll(seen);
        return result;
    }
}
