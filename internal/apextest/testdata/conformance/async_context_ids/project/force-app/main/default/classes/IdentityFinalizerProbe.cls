public class IdentityFinalizerProbe implements Finalizer {
    public void execute(FinalizerContext context) {
        String assigned = context.getAsyncApexJobId();
        Id typed = context.getAsyncApexJobId();
        String viaId = typed;
        String concatenated = '' + context.getAsyncApexJobId();
        String valueOf = String.valueOf(context.getAsyncApexJobId());
        String formatted = String.format('{0}', new List<Object>{context.getAsyncApexJobId()});
        insert new Account(Name = 'finalizer', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
    }
}
