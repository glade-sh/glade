Map<String, Schema.SObjectField> beforeFields = Schema.SObjectType.ObjectPermissions.fields.getMap();
Integer beforeCount = beforeFields.size();
Boolean beforeKey = beforeFields.containsKey('key');
String key = 'a';
ObjectPermissions first = new ObjectPermissions(SobjectType = (key == 'a' ? 'Account' : 'Contact'), PermissionsRead = true);
key = 'b';
ObjectPermissions second = new ObjectPermissions(SobjectType = key == 'a' ? 'Account' : 'Contact', PermissionsRead = false);
Map<String, Schema.SObjectField> afterFields = Schema.SObjectType.ObjectPermissions.fields.getMap();

System.assertEquals(false, beforeKey);
System.assertEquals(false, afterFields.containsKey('key'));
System.assertEquals(false, afterFields.containsKey('getMap'));
System.assertEquals(beforeCount, afterFields.size());
System.assertEquals('Account', first.SobjectType);
System.assertEquals('Contact', second.SobjectType);
System.assertEquals(true, first.PermissionsRead);
System.assertEquals(false, second.PermissionsRead);
