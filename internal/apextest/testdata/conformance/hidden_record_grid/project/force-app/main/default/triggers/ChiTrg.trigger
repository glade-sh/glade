trigger ChiTrg on Chi__c (after insert, after update) {
    String ev = Trigger.isInsert ? 'I' : 'U';
    for (Chi__c c : Trigger.new) {
        Id pid = c.Parent__c;
        List<Par__c> direct = [SELECT Cnt__c, SumAmt__c FROM Par__c WHERE Id = :pid LIMIT 1];
        List<Chi__c> rel = [SELECT Parent__r.Cnt__c, Parent__r.SumAmt__c FROM Chi__c WHERE Id = :c.Id LIMIT 1];
        String d = direct.isEmpty() ? 'none' : ('c=' + direct[0].Cnt__c + ',s=' + direct[0].SumAmt__c);
        String r = rel.isEmpty() ? 'none' : ('c=' + rel[0].Parent__r.Cnt__c + ',s=' + rel[0].Parent__r.SumAmt__c);
        System.debug('OBS|TRIG|' + ev + '|direct=' + d + '|rel=' + r);
    }
}
