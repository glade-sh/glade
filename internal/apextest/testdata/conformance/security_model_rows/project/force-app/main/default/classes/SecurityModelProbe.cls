public class SecurityModelProbe {
 public static Map<String,String> rows() {
  Map<String,String> r=new Map<String,String>();
  Id pid=UserInfo.getProfileId();
  User actor=[SELECT Profile.Name FROM User WHERE Id=:UserInfo.getUserId()];
  r.put('user_profile',actor.Profile.Name);
  List<PermissionSet> profiles=[SELECT Id,Name,Label,Profile.Name FROM PermissionSet WHERE ProfileId=:pid];
  r.put('profile_set_count',String.valueOf(profiles.size()));
  PermissionSet own=profiles[0];
  r.put('profile_set_label_opaque',String.valueOf(own.Label!=null && own.Label!=own.Profile.Name));
  r.put('profile_set_name_differs',String.valueOf(own.Name!=own.Profile.Name));
  r.put('profile_set_stable',String.valueOf(own.Id==[SELECT Id FROM PermissionSet WHERE ProfileId=:pid].Id));
  PermissionSetGroup grp=[SELECT Id,MasterLabel FROM PermissionSetGroup WHERE DeveloperName='SecurityBundle'];
  Id gid=grp.Id;
  r.put('group_label',grp.MasterLabel);
  r.put('group_components',String.valueOf([SELECT COUNT() FROM PermissionSetGroupComponent WHERE PermissionSetGroupId=:grp.Id AND PermissionSet.Name='SecurityReader']));
  Set<Id> psIds=new Set<Id>();
  for(PermissionSet ps:[SELECT Id FROM PermissionSet WHERE Name='SecurityReader' OR PermissionSetGroupId=:grp.Id]) psIds.add(ps.Id);
  List<PermissionSet> grouped=[SELECT Label,Type FROM PermissionSet WHERE PermissionSetGroupId=:grp.Id AND Type='Group'];
  r.put('group_set_count',String.valueOf(grouped.size()));
  r.put('group_set_label',grouped[0].Label);
  Set<Id> entities=new Set<Id>();
  for(ApexClass c:[SELECT Id FROM ApexClass WHERE Name IN ('SecurityEntry','SecurityUnlisted')]) entities.add(c.Id);
  for(ApexPage p:[SELECT Id FROM ApexPage WHERE Name IN ('SecurityLanding','SecurityUnlistedPage')]) entities.add(p.Id);
  for(String mode:new List<String>{'all','profile','regular','group'}) {
   String condition='';
   if(mode=='profile') condition=' AND Parent.IsOwnedByProfile=true';
   if(mode=='regular') condition=' AND Parent.IsOwnedByProfile=false AND Parent.PermissionSetGroupId=null';
   if(mode=='group') condition=' AND Parent.IsOwnedByProfile=false AND Parent.PermissionSetGroupId!=null';
   List<String> lines=new List<String>();
   for(SetupEntityAccess a:Database.query('SELECT Parent.Label,Parent.Profile.Name,Parent.IsOwnedByProfile,Parent.PermissionSetGroupId,SetupEntityType,SetupEntityId FROM SetupEntityAccess WHERE (Parent.ProfileId=:pid OR ParentId IN :psIds) AND (SetupEntityType=\'ApexClass\' OR SetupEntityType=\'ApexPage\') AND SetupEntityId IN :entities'+condition+' ORDER BY Parent.Profile.Name,Parent.Label')) {
    lines.add(a.SetupEntityType+'|'+(a.Parent.Profile.Name==null?a.Parent.Label:'<profile>')+'|'+a.Parent.Profile.Name+'|'+a.Parent.IsOwnedByProfile+'|'+(a.Parent.PermissionSetGroupId!=null));
   }
   lines.sort(); r.put('setup_'+mode,String.join(lines,';'));
  }
  for(String mode:new List<String>{'profile','regular','group','sourceProfile'}) {
   String condition=mode=='sourceProfile'?'Parent.Profile.Name=\'SecurityAuditor\'':mode=='profile'?'Parent.ProfileId=:pid':(mode=='regular'?'Parent.Name=\'SecurityReader\' AND Parent.PermissionSetGroupId=null':'Parent.PermissionSetGroupId=:gid AND Parent.Type=\'Group\'');
   List<String> lines=new List<String>();
   for(ObjectPermissions a:Database.query('SELECT Parent.Label,Parent.Profile.Name,SobjectType,PermissionsRead,PermissionsCreate,PermissionsEdit,PermissionsDelete,PermissionsViewAllRecords,PermissionsModifyAllRecords FROM ObjectPermissions WHERE SobjectType=\'SecuritySample__c\' AND '+condition)) lines.add((a.Parent.Profile.Name==null?a.Parent.Label:'<profile>')+'|'+a.Parent.Profile.Name+'|'+a.SobjectType+'|'+a.PermissionsRead+'|'+a.PermissionsCreate+'|'+a.PermissionsEdit+'|'+a.PermissionsDelete+'|'+a.PermissionsViewAllRecords+'|'+a.PermissionsModifyAllRecords);
   lines.sort(); r.put('object_'+mode,String.join(lines,';'));
   lines=new List<String>();
   for(FieldPermissions a:Database.query('SELECT Parent.Label,Parent.Profile.Name,SobjectType,Field,PermissionsRead,PermissionsEdit FROM FieldPermissions WHERE SobjectType=\'SecuritySample__c\' AND '+condition)) lines.add((a.Parent.Profile.Name==null?a.Parent.Label:'<profile>')+'|'+a.Parent.Profile.Name+'|'+a.Field+'|'+a.PermissionsRead+'|'+a.PermissionsEdit);
   lines.sort(); r.put('field_'+mode,String.join(lines,';'));
  }
  r.put('source_profile_set_count',String.valueOf([SELECT COUNT() FROM PermissionSet WHERE Profile.Name='SecurityAuditor']));
  r.put('source_profile_setup_count',String.valueOf([SELECT COUNT() FROM SetupEntityAccess WHERE Parent.Profile.Name='SecurityAuditor' AND (SetupEntityType='ApexClass' OR SetupEntityType='ApexPage') AND SetupEntityId IN :entities]));
  List<String> types=new List<String>();
  for(AggregateResult a:[SELECT SetupEntityType,COUNT(Id) n FROM SetupEntityAccess WHERE Parent.Name='SecurityReader' GROUP BY SetupEntityType]) types.add(String.valueOf(a.get('SetupEntityType'))+'|'+a.get('n'));
  types.sort(); r.put('declared_setup_types',String.join(types,';'));
  List<String> tabs=new List<String>();
  for(PermissionSetTabSetting a:[SELECT Name,Visibility FROM PermissionSetTabSetting WHERE Parent.Name='SecurityReader' AND Name='SecuritySample__c']) tabs.add(a.Name+'|'+a.Visibility);
  r.put('tab_regular',String.join(tabs,';'));
  r.put('metadata_entity_count',String.valueOf([SELECT COUNT() FROM SetupEntityAccess WHERE Parent.Name='SecurityReader' AND SetupEntityType='CustomEntityDefinition']));
  return r;
 }
}
