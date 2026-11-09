@IsTest private class ProfileProbe {
 @IsTest static void capture() {
  List<Object> output = new List<Object>();
  List<Profile> profiles = [SELECT Id, Name, UserType FROM Profile WHERE UserType = 'Standard'];
  for (Profile p : profiles) {
   Map<String,Object> row = new Map<String,Object>{'name'=>p.Name};
   List<PermissionSet> owners = [SELECT Id FROM PermissionSet WHERE IsOwnedByProfile=true AND ProfileId=:p.Id];
   row.put('ownerCount',owners.size());
   List<Object> grants = new List<Object>();
   for (ObjectPermissions op : [SELECT PermissionsRead, PermissionsCreate, PermissionsEdit, PermissionsDelete, PermissionsViewAllRecords, PermissionsModifyAllRecords FROM ObjectPermissions WHERE ParentId IN :owners AND SObjectType='Account']) {
    grants.add(new List<Boolean>{op.PermissionsRead,op.PermissionsCreate,op.PermissionsEdit,op.PermissionsDelete,op.PermissionsViewAllRecords,op.PermissionsModifyAllRecords});
   }
   row.put('grants',grants);
   try {
    User u = new User(Alias='probe',Email='probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=p.Id,Username='probe.'+String.valueOf(Crypto.getRandomInteger())+'@example.invalid');
    System.runAs(u) { row.put('account',Schema.sObjectType.Account.isAccessible()); row.put('nameField',Schema.sObjectType.Account.fields.Name.isAccessible()); }
   } catch (Exception e) { row.put('error',e.getTypeName()+': '+e.getMessage()); }
   output.add(row);
  }
  List<String> matches = new List<String>();
  for(PermissionSet ps : [SELECT Profile.Name FROM PermissionSet WHERE IsOwnedByProfile=true AND Profile.UserType='Standard' AND Id NOT IN (SELECT ParentId FROM ObjectPermissions WHERE SobjectType='Account' AND PermissionsRead=true)]) { matches.add(ps.Profile.Name); }
  List<PermissionSet> first = [SELECT Profile.Name FROM PermissionSet WHERE IsOwnedByProfile=true AND Profile.UserType='Standard' AND Id NOT IN (SELECT ParentId FROM ObjectPermissions WHERE SobjectType='Account' AND PermissionsRead=true) LIMIT 1];
  System.assert(false,'PROFILE_CAPTURE:'+JSON.serialize(new Map<String,Object>{'profiles'=>output,'matches'=>matches,'first'=>first.isEmpty()?null:first[0].Profile.Name}));
 }
}
