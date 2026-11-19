trigger ChiTrg on Chi__c (after insert, after update) {
 for (Chi__c c : Trigger.new) {
  Par__c p = [SELECT Cnt__c, SumAmt__c FROM Par__c WHERE Id=:c.Parent__c];
  Chi__c r = [SELECT Parent__r.Cnt__c, Parent__r.SumAmt__c FROM Chi__c WHERE Id=:c.Id];
  ResProbe.seen.add((Trigger.isInsert?'insert':'update')+':'+c.ExtId__c+':direct='+p.Cnt__c+','+p.SumAmt__c+':rel='+r.Parent__r.Cnt__c+','+r.Parent__r.SumAmt__c);
 }
}
