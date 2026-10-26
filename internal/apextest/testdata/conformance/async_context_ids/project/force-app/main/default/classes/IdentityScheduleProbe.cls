public class IdentityScheduleProbe implements Schedulable {
    public void execute(SchedulableContext context) {
        String assigned = context.getTriggerId();
        Id typed = context.getTriggerId();
        String viaId = typed;
        String concatenated = '' + context.getTriggerId();
        String valueOf = String.valueOf(context.getTriggerId());
        String formatted = String.format('{0}', new List<Object>{context.getTriggerId()});
        insert new Account(Name = 'scheduled', Phone = viaId, Description = JSON.serialize(new List<Object>{assigned.length(), concatenated.length(), valueOf.length(), viaId.length(), formatted.length(), assigned == viaId, concatenated == viaId, valueOf == viaId, formatted == viaId}));
    }
}
