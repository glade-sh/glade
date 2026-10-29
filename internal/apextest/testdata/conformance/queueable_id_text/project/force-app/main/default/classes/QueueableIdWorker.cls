public class QueueableIdWorker implements Queueable {
    private Boolean captureContext;
    public QueueableIdWorker(Boolean captureContext) { this.captureContext = captureContext; }
    public void execute(QueueableContext context) {
        if (!captureContext) return;
        String contextIdText = context.getJobId();
        System.assert(false, 'OBSERVED|observeContextIdText|' + JSON.serialize(new Map<String, Object>{'contextIdTextLength' => contextIdText.length()}));
    }
}
