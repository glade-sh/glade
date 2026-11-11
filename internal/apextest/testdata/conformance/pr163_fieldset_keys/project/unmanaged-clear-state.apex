Map<String, Schema.FieldSet> first = Account.SObjectType.getDescribe().fieldSets.getMap();
Map<String, Schema.FieldSet> second = Account.SObjectType.getDescribe().fieldSets.getMap();
first.clear();
Map<String, Schema.FieldSet> fresh = Account.SObjectType.getDescribe().fieldSets.getMap();
System.debug('CLEAR:' + JSON.serialize(new Map<String,Object>{'afterSize'=>first.size(),'afterContains'=>first.containsKey('CodexC163Capture'),'afterGet'=>first.get('CodexC163Capture') != null,'secondSize'=>second.size(),'secondContains'=>second.containsKey('CodexC163Capture'),'freshSize'=>fresh.size(),'freshContains'=>fresh.containsKey('CodexC163Capture')}));
