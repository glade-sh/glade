public class QueueCapacityProbe {
    public class EmptyWork implements Queueable {
        public void execute(QueueableContext context) {}
    }

    public class MeasureWork implements Queueable {
        public void execute(QueueableContext context) {
            Map<String, Object> observation = new Map<String, Object>{
                'beforeLimit' => Limits.getLimitQueueableJobs(),
                'beforeCount' => Limits.getQueueableJobs(),
                'isQueueable' => System.isQueueable()
            };
            System.enqueueJob(new EmptyWork());
            observation.put('afterLimit', Limits.getLimitQueueableJobs());
            observation.put('afterCount', Limits.getQueueableJobs());
            Boolean hasCapacity = Limits.getLimitQueueableJobs() > Limits.getQueueableJobs();
            observation.put('hasCapacity', hasCapacity);
            if (hasCapacity) {
                System.enqueueJob(new EmptyWork());
            }
            insert new Account(Name = 'Queue capacity observation', Description = JSON.serialize(observation));
        }
    }
}
