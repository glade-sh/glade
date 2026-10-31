Id business=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Business_Default' LIMIT 1].Id;
for(String route:new List<String>{'statement_insert','statement_upsert','list_insert','list_upsert'}) {
 for(String shape:new List<String>{'null_business_type','null_business_missing'}) {
  Account a=new Account(RecordTypeId=business); a.LastName=null;
  if(shape=='null_business_type') a.Name='Business';
  try {
   if(route=='statement_insert') { insert a; }
   else if(route=='statement_upsert') { upsert a; }
   else if(route=='list_insert') { List<Account> batch=new List<Account>{a}; insert batch; }
   else { List<Account> batch=new List<Account>{a}; upsert batch; }
  } catch(DmlException e) { System.debug('BOUNDARY_MESSAGE|'+route+'_'+shape+'|'+e.getDmlMessage(0)); }
 }
}
