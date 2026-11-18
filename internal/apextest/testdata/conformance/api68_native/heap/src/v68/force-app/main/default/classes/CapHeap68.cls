public class CapHeap68 {
    public static void run() {
        System.debug(LoggingLevel.ERROR, 'SYNC_HEAP_BYTES=' + Limits.getLimitHeapSize()
            + ';SYNC_HEAP_USED=' + Limits.getHeapSize());
    }
    public static void runAsync() {
        System.enqueueJob(new Worker68());
    }
    public class Worker68 implements Queueable {
        public void execute(QueueableContext ctx) {
            Integer cap = Limits.getLimitHeapSize();
            System.debug(LoggingLevel.ERROR, 'ASYNC_HEAP_BYTES=' + cap
                + ';ASYNC_HEAP_USED=' + Limits.getHeapSize());
            insert new Capture_Row__c(Case__c = 'heap', Stage__c = 'async68',
                Body__c = 'ASYNC_HEAP_BYTES=' + cap + ';ASYNC_HEAP_USED=' + Limits.getHeapSize());
        }
    }
}
