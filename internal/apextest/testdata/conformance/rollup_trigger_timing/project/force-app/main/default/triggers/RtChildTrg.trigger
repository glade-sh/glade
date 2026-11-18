trigger RtChildTrg on RtChild__c (before insert, before update, before delete, after insert, after update, after delete, after undelete) {
    String stage = String.valueOf(Trigger.isBefore).left(1) + String.valueOf(Trigger.isAfter).left(1);
    String ev;
    if (Trigger.isInsert) {
        ev = 'I';
    } else if (Trigger.isUpdate) {
        ev = 'U';
    } else if (Trigger.isDelete) {
        ev = 'D';
    } else {
        ev = 'N';
    }
    Integer n = 0;
    Set<Id> pids = new Set<Id>();
    if (Trigger.new != null) {
        n = Trigger.new.size();
        for (RtChild__c c : Trigger.new) {
            pids.add(c.Parent__c);
        }
    }
    if (Trigger.old != null) {
        if (n == 0) {
            n = Trigger.old.size();
        }
        for (RtChild__c c : Trigger.old) {
            pids.add(c.Parent__c);
        }
    }
    if (Trigger.isAfter && Trigger.isDelete) {
        RtLog.delCount++;
        RtLog.delSizes.add(String.valueOf(Trigger.old.size()));
    }
    List<RtParent__c> ps = [SELECT Count__c, SumAmt__c, MinAmt__c, MaxAmt__c, FiltSum__c, FiltCount__c FROM RtParent__c WHERE Id IN :pids LIMIT 1];
    System.debug('OBS|EV|' + stage + ev + n + '|' + RtFmt.of(ps[0]) + '|old=' + (Trigger.old == null ? -1 : Trigger.old.size()) + '|obs=' + RtLog.obs);
    RtLog.add(stage + ev + n + '|' + RtFmt.of(ps[0]));
}
