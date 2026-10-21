public with sharing class MxW40 {
    public static void apply(PrbLedger__c r, String mode) {
        if (mode == 'add') { r.Amount__c += 3; }
        else if (mode == 'sub') { r.Amount__c -= 3; }
        else if (mode == 'addx') { r.Amount__c = r.Amount__c + 3; }
        else if (mode == 'set') { r.Amount__c = 99; }
        else if (mode == 'txt') { r.Name += 'x'; }
        else if (mode == 'inc') { r.Count__c++; }
        else if (mode == 'cnt') { r.Count__c += 2; }
        else if (mode == 'put') { r.put('Amount__c', r.Amount__c + 3); }
        else if (mode == 'amtread') { Decimal d = r.Amount__c; }

    }
}