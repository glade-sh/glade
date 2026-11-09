Map<String, Schema.SObjectField> beforeFields = Schema.SObjectType.ObjectPermissions.fields.getMap();
Integer beforeCount = beforeFields.size();
Boolean beforeKey = beforeFields.containsKey('key');
String key = 'a';
ObjectPermissions first = new ObjectPermissions(SobjectType = (key == 'a' ? 'Account' : 'Contact'), PermissionsRead = true);
key = 'b';
ObjectPermissions second = new ObjectPermissions(SobjectType = key == 'a' ? 'Account' : 'Contact', PermissionsRead = false);
Map<String, Schema.SObjectField> afterFields = Schema.SObjectType.ObjectPermissions.fields.getMap();
System.debug('CASE375|' + JSON.serialize(new Map<String,Object>{'beforeKey'=>beforeKey,'afterKey'=>afterFields.containsKey('key'),'sameCount'=>beforeCount == afterFields.size(),'first'=>first.SobjectType,'second'=>second.SobjectType,'readTrue'=>first.PermissionsRead,'readFalse'=>second.PermissionsRead,'getMapField'=>afterFields.containsKey('getMap')}));
