for (String objectName : new List<String>{'Lead','Campaign','Contract','Asset','Product2','Pricebook2','Order','OpportunityLineItem'}) {
 try {
  List<SObject> rows=Database.query('SELECT Id,UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM '+objectName+' LIMIT 1');
  Id recordId=(Id)rows[0].get('Id'); Id userId=UserInfo.getUserId();
  List<UserRecordAccess> direct=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:userId AND RecordId=:recordId];
  SObject projection=rows[0].getSObject('UserRecordAccess'); List<Object> rel=new List<Object>(); List<Object> obj=new List<Object>();
  for (String f : new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}) { rel.add(projection.get(f)); obj.add(direct[0].get(f)); }
  System.debug('ROAD174C|'+objectName+'|ok|'+JSON.serialize(rel)+'|'+JSON.serialize(obj));
 } catch (Exception e) { System.debug('ROAD174C|'+objectName+'|reject|'+e.getTypeName()+':'+e.getMessage()); }
}
