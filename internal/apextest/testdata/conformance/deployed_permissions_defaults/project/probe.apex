Map<String,Schema.SObjectField> fs=Schema.SObjectType.PermissionSet.fields.getMap();
List<String> names=new List<String>();
for(String key:fs.keySet()){Schema.DescribeFieldResult f=fs.get(key).getDescribe(); if(key.startsWith('permissions') && f.getType()==Schema.DisplayType.BOOLEAN && f.isAccessible()) names.add(f.getName());}
names.sort();
System.debug('C27FIELDS|'+String.join(names,','));
List<SObject> rows=Database.query('SELECT Name,'+String.join(names,',')+' FROM PermissionSet WHERE Name IN (\'C27_Empty\',\'C27_Enabled\',\'C27_Disabled\') ORDER BY Name');
for(SObject row:rows){List<String> trues=new List<String>(); List<String> missingValues=new List<String>(); for(String n:names){Object v=row.get(n);if(v==null)missingValues.add(n);else if((Boolean)v)trues.add(n);} System.debug('C27CAPTURE|'+row.get('Name')+'|count='+names.size()+'|true='+String.join(trues,',')+'|null='+String.join(missingValues,','));}
