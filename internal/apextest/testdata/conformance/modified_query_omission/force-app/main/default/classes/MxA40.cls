public with sharing class MxA40 {
    public static String rd(PrbLedger__c r) {
        String s = '';
        try { Object v = r.Plan__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.get('Plan__c'); s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Plan__r; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        try { Object v = r.Note__c; s += v == null ? 'N' : 'V'; } catch (Exception e) { s += 'E'; }
        return s;
    }

    public static String run(Id rid, String mode, Boolean dyn) {
        PrbLedger__c r;
        if (dyn) { r = Database.query('SELECT Id,Name,Amount__c,Count__c FROM PrbLedger__c WHERE Id = :rid'); }
        else { r = [SELECT Id,Name,Amount__c,Count__c FROM PrbLedger__c WHERE Id = :rid]; }
        if (mode == 'add') { r.Amount__c += 3; }
        else if (mode == 'sub') { r.Amount__c -= 3; }
        else if (mode == 'addx') { r.Amount__c = r.Amount__c + 3; }
        else if (mode == 'set') { r.Amount__c = 99; }
        else if (mode == 'txt') { r.Name += 'x'; }
        else if (mode == 'inc') { r.Count__c++; }
        else if (mode == 'cnt') { r.Count__c += 2; }
        else if (mode == 'put') { r.put('Amount__c', r.Amount__c + 3); }
        else if (mode == 'amtread') { Decimal d = r.Amount__c; }

        return rd(r);
    }
}