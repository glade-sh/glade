@IsTest private class Road174ViewOnly62 {

 private static List<Object> vector(SObject s) { List<Object> a=new List<Object>(); for(String f:new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}) a.add(s.get(f));return a; }
 private static void observe(String mode) {
  Map<String,Object> out=new Map<String,Object>();
  try {
   Account a=new Account(Name='Profile proof');insert a;
   Order other=new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft');insert other;
   Profile p=[SELECT Id,Name FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1]; out.put('actor_profile',p.Name);
   User actor=new User(Username='road174profile.'+mode+'.'+System.now().getTime()+'@example.invalid',Alias='prf',Email='proof@example.invalid',LastName='Proof',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
   System.runAs(new User(Id=UserInfo.getUserId())) {
    insert actor;
    PermissionSet ps=new PermissionSet(Name='Road174All_'+mode,Label='All data proof');
    Database.SaveResult made=Database.insert(ps,false); if(!made.isSuccess()){out.put('setup_error','permset:'+made.getErrors()[0].getStatusCode()+':'+made.getErrors()[0].getMessage());System.assert(false,'ROAD174P|'+mode+'|'+JSON.serialize(out));}
    insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Account',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true);
    Database.SaveResult grant=Database.insert(new ObjectPermissions(ParentId=ps.Id,SObjectType='Order',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true,PermissionsViewAllRecords=true,PermissionsModifyAllRecords=false),false);
    out.put('object_grant',grant.isSuccess()?'ok':'reject:'+grant.getErrors()[0].getStatusCode());
    insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=ps.Id);
   }
   String suffix=mode=='default'?'':(' WITH '+mode+'_MODE');
   System.runAs(actor) {
    Order own=new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft');insert own;
    for(String ownership:new List<String>{'own','other'}){
     Id rid=ownership=='own'?own.Id:other.Id;Id uid=UserInfo.getUserId();
     try {
      List<SObject> q=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE Id=:rid'+suffix);
      List<UserRecordAccess> direct=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid];
      out.put(ownership+'_relationship',q.isEmpty()?null:vector(q[0].getSObject('UserRecordAccess')));out.put(ownership+'_direct',direct.isEmpty()?null:vector(direct[0]));
     }catch(Exception e){out.put(ownership+'_error',e.getTypeName()+':'+e.getMessage());}
    }
   }
  }catch(Exception e){out.put('setup_error',e.getTypeName()+':'+e.getMessage());}
  System.assert(false,'ROAD174P|'+mode+'|'+JSON.serialize(out));
 }
 @IsTest static void testDefault(){observe('default');}
 @IsTest static void testSystem(){observe('SYSTEM');}
 @IsTest static void testUser(){observe('USER');}
}
