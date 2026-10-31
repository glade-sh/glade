Schema.DescribeFieldResult d = User.LocaleSidKey.getDescribe();
List<Object> entries = new List<Object>();
for (Schema.PicklistEntry e : d.getPicklistValues()) entries.add(new Map<String,Object>{'value'=>e.getValue(),'label'=>e.getLabel(),'active'=>e.isActive(),'defaultValue'=>e.isDefaultValue()});
System.debug('LOCALE_ENTRIES|' + JSON.serialize(entries));
System.debug('LOCALE_RESTRICTED|' + d.isRestrictedPicklist());
Savepoint sp = Database.setSavepoint();
for (String v : new List<String>{'en_US','pl_PL','af','haw','en','az','NotAnAllowedValue'}) {
 Database.SaveResult r = Database.update(new User(Id=UserInfo.getUserId(),LocaleSidKey=v),false);
 Map<String,Object> row = new Map<String,Object>{'value'=>v,'success'=>r.isSuccess()};
 if (r.isSuccess()) row.put('stored',[SELECT LocaleSidKey FROM User WHERE Id=:UserInfo.getUserId()].LocaleSidKey);
 else { Database.Error e=r.getErrors()[0]; row.put('status',String.valueOf(e.getStatusCode())); row.put('message',e.getMessage()); row.put('fields',e.getFields()); }
 System.debug('LOCALE_DML|' + JSON.serialize(row));
}
Database.rollback(sp);
