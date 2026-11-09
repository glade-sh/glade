 SObject createRecord(String obj, Id accountId, String label) {
  SObject r=Schema.getGlobalDescribe().get(obj).newSObject();
  if(obj=='Contract') { r.put('AccountId',accountId); r.put('StartDate',Date.today()); r.put('ContractTerm',12); r.put('Description',label); }
  if(obj=='Asset') { r.put('Name',label); r.put('AccountId',accountId); }
  if(obj=='Order') { r.put('AccountId',accountId); r.put('EffectiveDate',Date.today()); r.put('Status','Draft'); r.put('Description',label); }
  if(obj=='Pricebook2') { r.put('Name',label); r.put('IsActive',true); }
  if(obj=='Account') { r.put('Name',label); }
  if(obj=='Opportunity') { r.put('Name',label); r.put('AccountId',accountId); r.put('StageName','Prospecting'); r.put('CloseDate',Date.today().addDays(30)); }
  insert r; return r;
 }
 String share(String obj, Id recId, Id userId, String access) {
  Schema.SObjectType st=Schema.getGlobalDescribe().get(obj+'Share');
  if(st==null) return 'noshare';
  try {
   SObject s=st.newSObject();
   Map<String,Schema.SObjectField> f=st.getDescribe().fields.getMap();
   s.put(f.containsKey(obj.toLowerCase()+'id')?obj+'Id':'ParentId',recId);
   s.put('UserOrGroupId',userId);
   s.put(f.containsKey(obj.toLowerCase()+'accesslevel')?obj+'AccessLevel':'AccessLevel',access);
   if(obj=='Account') { s.put('OpportunityAccessLevel','None'); s.put('CaseAccessLevel','None'); }
   Database.SaveResult r=Database.insert(s,false);
   return r.isSuccess()?'ok':'reject:'+String.valueOf(r.getErrors()[0].getStatusCode());
  } catch(Exception e) { return 'error:'+e.getTypeName()+':'+e.getMessage(); }
 }
User r=[SELECT Id FROM User WHERE LastName='LC2Anon' LIMIT 1];
Map<String,Object> ids=new Map<String,Object>();
Account a=new Account(Name='LC2A parent'); insert a;
ids.put('acc_share',share('Account',a.Id,r.Id,'Read'));
for(String obj:new List<String>{'Asset','Contract','Order','Pricebook2','Account','Opportunity'}) {
 SObject other=createRecord(obj,a.Id,'LC2A other'); ids.put(obj+'|other',other.Id);
 if(obj!='Pricebook2') {
  ids.put(obj+'|other_share',share(obj,other.Id,r.Id,'Edit'));
  try {
   SObject own=Schema.getGlobalDescribe().get(obj).newSObject();
   SObject tmpl=other.clone(false,true);
   tmpl.put('OwnerId',r.Id);
   insert tmpl; ids.put(obj+'|own',tmpl.Id);
  } catch(Exception e) { ids.put(obj+'|own','reject:'+e.getTypeName()+':'+e.getMessage()); }
 }
}
System.debug('LC2IDS|'+JSON.serialize(ids));
