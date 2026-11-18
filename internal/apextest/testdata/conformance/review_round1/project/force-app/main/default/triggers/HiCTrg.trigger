trigger HiCTrg on HiC__c (after insert) {
  if (!HiLog.nested) { HiLog.nested = true; insert new HiC__c(Name = 'inner', P__c = Trigger.new[0].P__c); }
  HiLog.seen += String.valueOf([SELECT Cnt__c FROM HiP__c WHERE Id = :Trigger.new[0].P__c].Cnt__c) + ',';
}
