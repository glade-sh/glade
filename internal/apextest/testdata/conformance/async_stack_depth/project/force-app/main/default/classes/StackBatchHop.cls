public class StackBatchHop implements Database.Batchable<Integer> {
        String token; Integer hop; Integer count; Boolean mixed;
        public StackBatchHop(String token, Integer hop, Integer count, Boolean mixed) {
            this.token=token; this.hop=hop; this.count=count; this.mixed=mixed;
        }
        public Iterable<Integer> start(Database.BatchableContext context) { return new List<Integer>{1}; }
        public void execute(Database.BatchableContext context, List<Integer> scope) { }
        public void finish(Database.BatchableContext context) {
            String value=String.valueOf(hop)+':batch';
            if (hop < count) {
                try {
                    if (mixed) { System.enqueueJob(new StackDepthProbe.QueueHop(token, hop+1, count, 'plain', true, false)); }
                    else { Database.executeBatch(new StackBatchHop(token, hop+1, count, false), 1); }
                    value += '|accepted';
                } catch (Exception e) { value += '|'+e.getTypeName()+'|'+e.getMessage(); }
            }
            StackDepthProbe.save(token, value);
        }
    }
