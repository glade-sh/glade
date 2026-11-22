@IsTest private class KOrdQ621Own68 {
@IsTest static void observe() {
Map<String, Schema.SObjectType> objects = Schema.getGlobalDescribe();
String objectName = KOrdQ621__c.SObjectType.getDescribe().getName();
Map<String, Schema.SObjectField> fields = KOrdQ621__c.SObjectType.getDescribe().fields.getMap();
String fieldName = KOrdQ621__c.Alpha__c.getDescribe().getName();
Map<String, Schema.SObjectField> fresh = KOrdQ621__c.SObjectType.getDescribe().fields.getMap();
Map<String, Schema.SObjectField> copy = fields.clone();
Map<String, Schema.SObjectField> standard = Account.SObjectType.getDescribe().fields.getMap();
String result = objectName + '|' + fieldName;
result += '|' + objects.containsKey(objectName) + '|' + objects.containsKey('KOrdQ621__c');
result += '|' + fields.containsKey(fieldName) + '|' + fields.containsKey('Alpha__c') + '|' + fields.containsKey('Name');
result += '|' + standard.containsKey('Name') + '|' + standard.containsKey('DefinitelyMissing__c');
String fieldKey = '';
for (String key : fields.keySet()) if (key.equalsIgnoreCase(fieldName)) fieldKey = key;
Boolean removed = fields.keySet().remove(fieldKey);
result += '|' + removed + '|' + fields.containsKey(fieldName) + '|' + fresh.containsKey(fieldName) + '|' + copy.containsKey(fieldName);
String objectKey = '';
for (String key : objects.keySet()) if (key.equalsIgnoreCase(objectName)) objectKey = key;
Boolean globalRemoved = objects.keySet().remove(objectKey);
result += '|' + globalRemoved + '|' + objects.containsKey(objectName) + '|' + Schema.getGlobalDescribe().containsKey(objectName);
System.assertEquals('capture', result);

} }
