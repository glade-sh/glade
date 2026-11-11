trigger FernMetadataTrigger on FernProbe__c (after insert) {
 for (FernProbe__c row : Trigger.new) {
  Boolean active;
  if (row.Selector__c == 'Enabled') active = FernSwitch__mdt.getInstance('Enabled').Enabled__c;
  else if (row.Selector__c == 'Disabled') active = FernSwitch__mdt.getInstance('Disabled').Enabled__c;
  else active = FernSwitch__mdt.getInstance('Absent').Enabled__c;
  if (active) FernMetadataLog.hits++;
 }
}
