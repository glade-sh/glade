public class TailExecutionProbe {
    public class WorkException extends Exception {}
    public class Work implements System.Queueable {
        String mode;
        public Work(String mode) { this.mode = mode; }
        public void execute(System.QueueableContext context) {
            Tail tail = new Tail(mode);
            System.attachFinalizer(tail);
            insert new Account(Name = 'Tail observer body');
            tail.state = 'after attach';
            if (mode == 'failure') throw new WorkException('body interrupted');
            if (mode == 'duplicate') {
                try { System.attachFinalizer(new Tail('second')); }
                catch (Exception error) { tail.state = error.getTypeName() + '|' + error.getMessage(); }
            }
        }
    }
    public class Tail implements System.Finalizer {
        String mode;
        String state = 'before attach';
        public Tail(String mode) { this.mode = mode; }
        public void execute(System.FinalizerContext context) {
            Map<String, Object> out = new Map<String, Object>{
                'state' => state, 'result' => String.valueOf(context.getResult()),
                'dml' => Limits.getDmlStatements(), 'queueable' => System.isQueueable(),
                'bodyCount' => [SELECT COUNT() FROM Account WHERE Name = 'Tail observer body']
            };
            Id parentId = context.getAsyncApexJobId();
            out.put('parentStatus', [SELECT Status FROM AsyncApexJob WHERE Id = :parentId].Status);
            Exception error = context.getException();
            if (error != null) out.put('error', error.getTypeName() + '|' + error.getMessage());
            insert new Account(Name = 'Tail observer finalizer', Description = JSON.serialize(out));
            if (mode == 'child') System.enqueueJob(new Child());
        }
    }
    public class Child implements System.Queueable {
        public void execute(System.QueueableContext context) {
            insert new Account(Name = 'Tail observer child', Description = String.valueOf([SELECT COUNT() FROM Account WHERE Name = 'Tail observer finalizer']));
        }
    }
    public static String report() {
        Map<String, Object> out = new Map<String, Object>();
        for (Account row : [SELECT Name, Description FROM Account WHERE Name LIKE 'Tail observer %']) {
            if (row.Name == 'Tail observer finalizer') out.put('finalizer', JSON.deserializeUntyped(row.Description));
            if (row.Name == 'Tail observer child') out.put('child', row.Description);
        }
        out.put('bodyCount', [SELECT COUNT() FROM Account WHERE Name = 'Tail observer body']);
        return JSON.serialize(out);
    }
}
