public class IdentityQueueProbe implements Queueable {
    public void execute(QueueableContext context) {
        String assigned = context.getJobId();
        Id typed = context.getJobId();
        String viaId = typed;
        String concatenated = '' + context.getJobId();
        String valueOf = String.valueOf(context.getJobId());
        String formatted = String.format('{0}', new List<Object>{context.getJobId()});
        insert new Account(Name = 'queueable', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
        System.attachFinalizer(new IdentityFinalizerProbe());
    }
}
