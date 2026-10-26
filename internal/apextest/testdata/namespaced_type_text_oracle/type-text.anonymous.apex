Map<String, Object> observations = new Map<String, Object>();
observations.put('provider', String.valueOf(new TypeTextStubProvider()));
Object source = 'x';
try { TypeTextAnchor result = (TypeTextAnchor)source; observations.put('cast', 'NO_ERROR'); } catch (Exception e) { observations.put('cast', e.getTypeName() + '|' + e.getMessage()); }
Object listSource = new List<String>{'x'};
try { List<TypeTextAnchor> result = (List<TypeTextAnchor>)listSource; observations.put('listCast', 'NO_ERROR'); } catch (Exception e) { observations.put('listCast', e.getTypeName() + '|' + e.getMessage()); }
Map<String, Schema.SObjectType> types = Schema.getGlobalDescribe();
List<String> keys = new List<String>();
for (String key : types.keySet()) if (key.toLowerCase().contains('typetextitem')) keys.add(key);
keys.sort(); observations.put('keys', String.join(keys, '|'));
Schema.SObjectType found = types.get('TypeTextItem__c'); observations.put('get', found == null ? 'null' : found.getDescribe().getName());
observations.put('contains', types.containsKey('TypeTextItem__c'));
List<String> values = new List<String>();
for (Schema.FieldSetMember member : Thing__c.SObjectType.getDescribe().fieldSets.getMap().get('nsx__Shown').getFields()) values.add(member.getFieldPath() + '|' + member.getSObjectField().getDescribe().getName() + '|' + member.getLabel());
observations.put('fieldsetDescribeMap', String.join(values, ';'));
values.clear();
for (Schema.FieldSetMember member : Schema.SObjectType.Thing__c.FieldSets.getMap().get('nsx__Shown').getFields()) values.add(member.getFieldPath() + '|' + member.getSObjectField().getDescribe().getName() + '|' + member.getLabel());
observations.put('fieldsetTokenMap', String.join(values, ';'));
System.assert(false, 'P|T001|' + JSON.serialize(observations));
