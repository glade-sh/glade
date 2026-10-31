@IsTest private class MeadowModeTest {
 @IsTest static void observe() {
  String absent = null; String blank = '';
  System.assertEquals(1, [SELECT count() FROM MeadowGroup__mdt WHERE NamespacePrefix = :absent], 'null-filter');
  System.assertEquals(1, [SELECT count() FROM MeadowGroup__mdt WHERE NamespacePrefix = :blank], 'blank-filter');
  System.assertEquals(0, [SELECT count() FROM MeadowGroup__mdt WHERE DeveloperName = 'Missing'], 'missing-filter');
  List<MeadowGroup__mdt> rows = [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = 'MeadowEntry__c' WITH USER_MODE];
  System.assertEquals(1, rows.size(), 'user-mode-size');
  System.assertEquals('State__c', rows[0].Descriptor__r.Column__r.QualifiedApiName, 'user-mode-column');
  System.assertEquals(1, rows[0].Choices__r.size(), 'user-mode-child');
  System.assertEquals(0, [SELECT count() FROM MeadowGroup__mdt WHERE DeveloperName = 'Missing' WITH USER_MODE], 'user-mode-missing');
 }
}
