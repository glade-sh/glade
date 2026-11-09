PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='LC2Anon'];
insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Product2',PermissionsRead=true);
Database.SaveResult s=Database.insert(new ObjectPermissions(ParentId=ps.Id,SObjectType='Pricebook2',PermissionsRead=true),false);
System.debug('LC2GRANT|'+s.isSuccess()+'|'+(s.isSuccess()?'':String.valueOf(s.getErrors()[0].getStatusCode())));
