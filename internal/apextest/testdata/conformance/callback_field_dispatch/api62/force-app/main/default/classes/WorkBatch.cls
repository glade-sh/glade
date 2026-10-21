public class WorkBatch extends WorkUnit implements Queueable {
    public void execute(QueueableContext context) {
        this.callback = createCallback();
        System.attachFinalizer(this.callback);
    }
    public CompletionSink createCallback() { return new BatchCompletion(this); }
    private class BatchCompletion extends CompletionSink {
        private WorkBatch work;
        public BatchCompletion(WorkBatch work) { this.work = work; }
        protected override void handleSuccess() { this.work.finish(); }
    }
}
