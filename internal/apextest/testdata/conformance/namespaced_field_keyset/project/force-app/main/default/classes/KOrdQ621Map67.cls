@IsTest private class KOrdQ621Map67 {
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
Boolean removed = fields.keySet().remove(fieldName);
result += '|' + removed + '|' + fields.containsKey(fieldName) + '|' + fresh.containsKey(fieldName) + '|' + copy.containsKey(fieldName);
Boolean globalRemoved = objects.keySet().remove(objectName);
result += '|' + globalRemoved + '|' + objects.containsKey(objectName) + '|' + Schema.getGlobalDescribe().containsKey(objectName);
System.assertEquals('capture', result);

} }
