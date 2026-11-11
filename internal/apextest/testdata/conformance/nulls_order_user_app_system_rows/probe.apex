User u=[SELECT Id FROM User WHERE LastName='Synthetic App 0' LIMIT 1];
Id uid=u.Id;
List<UserAppInfo> rows=[SELECT Id,FormFactor,AppDefinitionId FROM UserAppInfo WHERE UserId=:uid];
Integer nullCount=0;
for(UserAppInfo r:rows) { if(r.AppDefinitionId==null) { nullCount++; } }
System.debug('CAPTURE|'+JSON.serialize(new Map<String,Object>{'savedCount'=>rows.size(),'nullRowCount'=>nullCount}));
