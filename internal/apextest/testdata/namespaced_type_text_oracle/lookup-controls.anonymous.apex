Map<String,Object> observations = new Map<String,Object>();
Map<String,Schema.SObjectType> allTypes=Schema.getGlobalDescribe();
observations.put('shareContains',allTypes.containsKey('TypeTextItem__Share'));
observations.put('shareGet',allTypes.get('TypeTextItem__Share') == null ? 'null' : allTypes.get('TypeTextItem__Share').getDescribe().getName());
observations.put('eventName',allTypes.get('nsx__TypeTextItem__ChangeEvent').getDescribe().getName());
observations.put('qualifiedGet',allTypes.get('NSX__TYPETEXTITEM__C').getDescribe().getName());
observations.put('foreignGet',allTypes.get('foreign__TypeTextItem__c') == null ? 'null' : 'found');
System.assert(false, 'P|T002|' + JSON.serialize(observations));
