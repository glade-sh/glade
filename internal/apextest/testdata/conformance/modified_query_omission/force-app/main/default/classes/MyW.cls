public with sharing class MyW {
    public static PrbLedger__c apply(PrbLedger__c r, String mode) {
        if (mode == 'add') { r.Amount__c += 3; }
        else if (mode == 'same') { r.Name = r.Name; }
        else if (mode == 'setun') { r.Note__c = 'z'; }
        else if (mode == 'putun') { r.put('Note__c', 'z'); }
        else if (mode == 'dml') { r.Amount__c += 3; update r; }
        else if (mode == 'dmlonly') { update r; }
        else if (mode == 'clone') { r.Amount__c += 3; return r.clone(true, true, false, true); }
        else if (mode == 'cloneclean') { PrbLedger__c c = r.clone(true, true, false, true); return c; }
        else if (mode == 'cloneAfter') { PrbLedger__c c = r.clone(true, true, false, true); c.Amount__c += 3; return c; }
        return r;
    }
}