Map<String,Schema.SObjectField> fs=GiftTransaction.SObjectType.getDescribe().fields.getMap();
for (String n : new List<String>{'GiftCommitmentScheduleId','OutreachSourceCodeId','PaymentInstrumentId'}) {
  Schema.SObjectType ref=fs.get(n).getDescribe().getReferenceTo()[0];
  try { System.debug('OWNEDREF '+n+'|'+String.valueOf(ref)+'|'+ref.getDescribe().getName()+'|'+Schema.getGlobalDescribe().containsKey(String.valueOf(ref))); }
  catch (Exception e) { System.debug('OWNEDREF '+n+'|ERROR|'+e.getTypeName()+'|'+e.getMessage()); }
}
