Map<String,Object> m = new Map<String,Object>();
Map<String,Schema.RecordTypeInfo> byDev = Schema.SObjectType.V35Rt__c.getRecordTypeInfosByDeveloperName();
Map<String,String> st = new Map<String,String>();
for (String n : new List<String>{'Alpha', 'Beta', 'Gamma'}) { st.put('before.' + n, byDev.get(n) == null ? 'null' : '' + byDev.get(n).isAvailable()); }
PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'RtGrant' LIMIT 1];
insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = ps.Id);
Map<String,Schema.RecordTypeInfo> byDev2 = Schema.SObjectType.V35Rt__c.getRecordTypeInfosByDeveloperName();
for (String n : new List<String>{'Alpha', 'Beta', 'Gamma'}) { st.put('after.' + n, byDev2.get(n) == null ? 'null' : '' + byDev2.get(n).isAvailable()); }
Schema.DescribeSObjectResult fresh = Schema.getGlobalDescribe().get('V35Rt__c').getDescribe();
for (String n : new List<String>{'Alpha', 'Beta', 'Gamma'}) { st.put('freshDescribe.' + n, fresh.getRecordTypeInfosByDeveloperName().get(n) == null ? 'null' : '' + fresh.getRecordTypeInfosByDeveloperName().get(n).isAvailable()); }
m.putAll(st);
System.debug('OBS|admin|' + JSON.serialize(m));
