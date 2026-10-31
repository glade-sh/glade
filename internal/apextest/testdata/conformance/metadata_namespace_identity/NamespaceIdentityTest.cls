@IsTest private class NamespaceIdentityTest {
 @IsTest static void observe() {
  System.assertEquals('gladens001__Enabled__c', gladens001__Ns113bFlag__mdt.gladens001__Enabled__c.getDescribe().getName());
  System.assertEquals('gladens001__Ns113bFlag__mdt', gladens001__Ns113bFlag__mdt.SObjectType.getDescribe().getName());
  List<FieldDefinition> found = [SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'gladens001__Ns113bFlag__mdt' AND QualifiedApiName = 'gladens001__Enabled__c'];
  System.assertEquals(1, found.size()); System.assertEquals('gladens001__Enabled__c', found[0].QualifiedApiName);
  List<FieldDefinition> missing = [SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'gladens001__Ns113bFlag__mdt' AND QualifiedApiName = 'gladens001__NoSuchField__c'];
  System.assertEquals(0, missing.size());
 }
}
