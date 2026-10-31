global class Road215NamespaceProbe {
 global static Map<String,Object> observe() {
  String objectName = MeadowEntry__c.SObjectType.getDescribe().getName();
  String fieldName = MeadowEntry__c.State__c.getDescribe().getName();
  MeadowField__mdt customField = [SELECT Entity__r.QualifiedApiName, Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'CustomState'];
  MeadowField__mdt standardField = [SELECT Entity__r.QualifiedApiName, Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'StandardRating'];
  List<MeadowGroup__mdt> groups = [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = :objectName];
  List<MeadowField__mdt> absent = [SELECT DeveloperName FROM MeadowField__mdt WHERE DeveloperName = 'Missing'];
  return new Map<String,Object>{'objectDescribe' => objectName, 'fieldDescribe' => fieldName, 'objectRelationship' => customField.Entity__r.QualifiedApiName, 'fieldRelationship' => customField.Column__r.QualifiedApiName, 'standardFieldRelationship' => standardField.Column__r.QualifiedApiName, 'matchingGroups' => groups.size(), 'missing' => absent.size()};
 }
}
