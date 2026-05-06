@isTest
private class CustomMetadataTest {
  @isTest static void queriesLoadedCustomMetadata() {
    List<Feature__mdt> rows = [SELECT DeveloperName, Enabled__c, Target__r.Description__c FROM Feature__mdt WHERE Target__r.QualifiedApiName = 'Target'];
    System.assertEquals(1, rows.size());
    System.assertEquals('Default', rows[0].DeveloperName);
    System.assert(rows[0].Enabled__c);
    System.assertEquals('Target row', rows[0].Target__r.Description__c);
  }
}
