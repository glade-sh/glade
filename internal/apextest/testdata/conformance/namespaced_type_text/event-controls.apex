Map<String,Object> observations = new Map<String,Object>();
Map<String,Schema.SObjectType> allTypes=Schema.getGlobalDescribe();
observations.put('eventLocalContains',allTypes.containsKey('TypeTextEventItem__ChangeEvent'));
observations.put('eventLocalGet',allTypes.get('TypeTextEventItem__ChangeEvent') == null ? 'null' : allTypes.get('TypeTextEventItem__ChangeEvent').getDescribe().getName());
observations.put('eventQualifiedContains',allTypes.containsKey('nsx__TypeTextEventItem__ChangeEvent'));
observations.put('eventQualifiedGet',allTypes.get('nsx__TypeTextEventItem__ChangeEvent').getDescribe().getName());
observations.put('eventLocalKey',allTypes.keySet().contains('typetexteventitem__changeevent'));
observations.put('eventQualifiedKey',allTypes.keySet().contains('nsx__typetexteventitem__changeevent'));
System.assert(false, 'P|T003|' + JSON.serialize(observations));
