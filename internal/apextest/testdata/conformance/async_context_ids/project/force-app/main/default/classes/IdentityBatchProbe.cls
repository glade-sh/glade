public class IdentityBatchProbe implements Database.Batchable<SObject> {
    public Iterable<SObject> start(Database.BatchableContext context) {
        String assigned = context.getJobId();
        Id typed = context.getJobId();
        String viaId = typed;
        String concatenated = '' + context.getJobId();
        String valueOf = String.valueOf(context.getJobId());
        String formatted = String.format('{0}', new List<Object>{context.getJobId()});
        insert new Account(Name = 'batchStart', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
        return new List<SObject>{new Account(Name = 'scope')};
    }
    public void execute(Database.BatchableContext context, List<SObject> scope) {
        String assigned = context.getJobId();
        Id typed = context.getJobId();
        String viaId = typed;
        String concatenated = '' + context.getJobId();
        String valueOf = String.valueOf(context.getJobId());
        String formatted = String.format('{0}', new List<Object>{context.getJobId()});
        insert new Account(Name = 'batchExecute', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
    }
    public void finish(Database.BatchableContext context) {
        String assigned = context.getJobId();
        Id typed = context.getJobId();
        String viaId = typed;
        String concatenated = '' + context.getJobId();
        String valueOf = String.valueOf(context.getJobId());
        String formatted = String.format('{0}', new List<Object>{context.getJobId()});
        insert new Account(Name = 'batchFinish', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
    }
}
