PermissionSet p=new PermissionSet(Name='GlobalApprovalGrant',Label='Global Approval Grant',PermissionsModifyAllData=true,PermissionsViewAllData=true);
Map<String,Schema.SObjectField> allFields=Schema.SObjectType.PermissionSet.fields.getMap();
for(Integer attempt=0;attempt<10;attempt++){
 Database.SaveResult r=Database.insert(p,false);
 if(r.isSuccess()){System.debug('GLOBAL_SETUP_OK');break;}
 String message=r.getErrors()[0].getMessage();Integer splitAt=message.indexOf('depends on permission(s):');
 if(splitAt<0){System.debug('GLOBAL_SETUP_FAIL|'+message);break;}
 String deps=message.substring(splitAt+'depends on permission(s):'.length());
 for(String token:deps.split(',')){String name='Permissions'+token.trim().replace('.','');if(allFields.containsKey(name)){p.put(name,true);}}
 if(attempt==9)System.debug('GLOBAL_SETUP_FAIL|dependency limit');
}
