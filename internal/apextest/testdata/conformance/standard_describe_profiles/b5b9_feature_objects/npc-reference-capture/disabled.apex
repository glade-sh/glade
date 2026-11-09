Map<String,Schema.SObjectType> ts=Schema.getGlobalDescribe();
for(String n:new List<String>{'GiftCommitmentSchedule','OutreachSourceCode','PaymentInstrument','Account'}) System.debug('OWNEDOFF '+n+'|'+ts.containsKey(n));
