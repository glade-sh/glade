trigger CedarRouteTrigger on Contact (before insert, before update, after insert, after update) {
    List<Contact> patches = new List<Contact>();
    for (Contact row : Trigger.new) {
        if (row.Description != null || row.Department == null) continue;
        Boolean beforeRoute = row.Department == 'before-invocable' || row.Department == 'before-interview';
        Boolean afterRoute = row.Department == 'after-invocable' || row.Department == 'after-interview';
        if ((Trigger.isBefore && beforeRoute) || (Trigger.isAfter && afterRoute)) {
            String result=CedarRouteProbe.invokeParent(row,row.Department.endsWith('invocable'));
            if (Trigger.isBefore) row.Description=result;
            else patches.add(new Contact(Id=row.Id,Description=result));
        }
    }
    if (!patches.isEmpty()) update patches;
}
