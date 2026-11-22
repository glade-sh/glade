@IsTest private class KOrdQ621Test62 {
@IsTest static void observe() {
Map<String, Schema.SObjectField> fields = KOrdQ621__c.SObjectType.getDescribe().fields.getMap();
Set<String> keys = fields.keySet();
String allText = String.valueOf(keys);
Set<String> keep = new Set<String>();
for (String key : keys) if (key.endsWith('__c')) keep.add(key);
keys.retainAll(keep);
String result = allText + '|' + String.valueOf(keys) + '|' + fields.size();
System.assertEquals('capture', result);
} }
