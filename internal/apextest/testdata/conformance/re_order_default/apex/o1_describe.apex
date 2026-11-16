Map<String,Schema.SObjectType> g=Schema.getGlobalDescribe();
List<String> gk=new List<String>(g.keySet());
System.debug('O1|DESC|GN|'+gk.size());
System.debug('O1|DESC|GIT|'+String.join(gk,','));
Map<String,Schema.SObjectField> af=g.get('Account').getDescribe().fields.getMap();
System.debug('O1|DESC|AIT|'+String.join(new List<String>(af.keySet()),','));
Map<String,Schema.SObjectField> ofm=g.get('Ord__c').getDescribe().fields.getMap();
System.debug('O1|DESC|OIT|'+String.join(new List<String>(ofm.keySet()),','));
