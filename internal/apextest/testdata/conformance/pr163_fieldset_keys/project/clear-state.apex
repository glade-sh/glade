Map<String, Schema.FieldSet> first = Account.SObjectType.getDescribe().fieldSets.getMap();
Map<String, Schema.FieldSet> second = Account.SObjectType.getDescribe().fieldSets.getMap();
first.clear();
Map<String, Schema.FieldSet> fresh = Account.SObjectType.getDescribe().fieldSets.getMap();
System.debug('CLEAR:' + JSON.serialize(new Map<String,Object>{'afterSize'=>first.size(),'afterContains'=>first.containsKey('gladetest1__CodexC163Capture'),'afterGet'=>first.get('gladetest1__CodexC163Capture') != null,'secondSize'=>second.size(),'secondContains'=>second.containsKey('gladetest1__CodexC163Capture'),'freshSize'=>fresh.size(),'freshContains'=>fresh.containsKey('gladetest1__CodexC163Capture')}));
