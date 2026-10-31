@IsTest private class Road174Matrix64 {

 private static SObject createRecord(String obj, Id accountId, String label) {
  SObject r=Schema.getGlobalDescribe().get(obj).newSObject();
  if(obj=='Contract') { r.put('AccountId',accountId); r.put('StartDate',Date.today()); r.put('ContractTerm',12); }
  if(obj=='Asset') { r.put('Name',label); r.put('AccountId',accountId); }
  if(obj=='Order') { r.put('AccountId',accountId); r.put('EffectiveDate',Date.today()); r.put('Status','Draft'); }
  if(obj=='Pricebook2') { r.put('Name',label); r.put('IsActive',true); }
  insert r; return r;
 }
 private static List<Object> vector(SObject s) {
  List<Object> result=new List<Object>();
  for(String f:new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}) result.add(s.get(f));
  return result;
 }
 private static void sample(String obj, String level) {
  Map<String,Object> payload=new Map<String,Object>();
  try {
   Account a=new Account(Name='Matrix account'); insert a;
   SObject other=createRecord(obj,a.Id,'Other record');
   Profile p=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
   User actor=new User(Username='road174.'+obj+'.'+level+'.'+System.now().getTime()+'@example.invalid',Alias='mtrx',Email='matrix@example.invalid',LastName='Matrix',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
   System.runAs(new User(Id=UserInfo.getUserId())) {
    insert actor;
    PermissionSet ps=new PermissionSet(Name='Matrix_'+obj+'_'+level,Label='Matrix'); insert ps;
    insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Account',PermissionsRead=true);
    ObjectPermissions op=new ObjectPermissions(ParentId=ps.Id,SObjectType=obj,PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=(level!='read'),PermissionsDelete=(level=='delete'));
    Database.SaveResult grant=Database.insert(op,false);
    payload.put('grant',grant.isSuccess()?'ok':'reject:'+String.valueOf(grant.getErrors()[0].getStatusCode())+':'+grant.getErrors()[0].getMessage());
    insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=ps.Id);
   }
   SObject own;
   System.runAs(actor) {
    try { own=createRecord(obj,a.Id,'Own record'); payload.put('own_insert','ok'); } catch(Exception e) { payload.put('own_insert','reject:'+e.getTypeName()); }
    for(String which:new List<String>{'other','own'}) {
     SObject target=which=='other'?other:own; if(target==null) continue;
     try {
      Id rid=(Id)target.get('Id'); Id uid=UserInfo.getUserId();
      List<SObject> q=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM '+obj+' WHERE Id=:rid');
      List<UserRecordAccess> direct=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid];
      payload.put(which+'_relationship',q.isEmpty()?null:vector(q[0].getSObject('UserRecordAccess')));
      payload.put(which+'_direct',direct.isEmpty()?null:vector(direct[0]));
     } catch(Exception e) { payload.put(which+'_error',e.getTypeName()+':'+e.getMessage()); }
    }
   }
   for(String which:new List<String>{'other','own'}) {
    SObject target=which=='other'?other:own; if(target==null) continue;
    try {
     Id rid=(Id)target.get('Id'); Id uid=UserInfo.getUserId();
     List<SObject> q=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM '+obj+' WHERE Id=:rid');
     List<UserRecordAccess> direct=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid];
     payload.put('admin_'+which+'_relationship',q.isEmpty()?null:vector(q[0].getSObject('UserRecordAccess')));
     payload.put('admin_'+which+'_direct',direct.isEmpty()?null:vector(direct[0]));
    } catch(Exception e) { payload.put('admin_'+which+'_error',e.getTypeName()+':'+e.getMessage()); }
   }
  } catch(Exception e) { payload.put('setup_error',e.getTypeName()+':'+e.getMessage()); }
  System.assert(false,'ROAD174M|'+obj+'|'+level+'|'+JSON.serialize(payload));
 }

 @IsTest static void testContractread() { sample('Contract','read'); }
 @IsTest static void testContractedit() { sample('Contract','edit'); }
 @IsTest static void testContractdelete() { sample('Contract','delete'); }
 @IsTest static void testAssetread() { sample('Asset','read'); }
 @IsTest static void testAssetedit() { sample('Asset','edit'); }
 @IsTest static void testAssetdelete() { sample('Asset','delete'); }
 @IsTest static void testPricebook2read() { sample('Pricebook2','read'); }
 @IsTest static void testPricebook2edit() { sample('Pricebook2','edit'); }
 @IsTest static void testPricebook2delete() { sample('Pricebook2','delete'); }
 @IsTest static void testOrderread() { sample('Order','read'); }
 @IsTest static void testOrderedit() { sample('Order','edit'); }
 @IsTest static void testOrderdelete() { sample('Order','delete'); }
}
