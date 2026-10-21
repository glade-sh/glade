public class CycleWorkerProbe extends CycleWorkerBaseProbe implements Queueable {
    public void execute(QueueableContext context) {
        this.callback = new StoredCallback(this);
        System.attachFinalizer(this.callback);
    }

    private class StoredCallback extends CycleCallbackProbe {
        private final CycleWorkerProbe owner;

        public StoredCallback(CycleWorkerProbe owner) {
            this.owner = owner;
        }

        public override void execute(FinalizerContext context) {
            this.owner.finish();
        }
    }
}
