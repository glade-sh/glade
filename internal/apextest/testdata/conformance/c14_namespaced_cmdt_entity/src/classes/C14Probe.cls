@IsTest
private class C14Probe {
  @IsTest static void counts() {
    System.assertEquals(2, C14Rule__mdt.getAll().size(), 'getAll');
    System.assertEquals(2, [SELECT COUNT() FROM C14Rule__mdt], 'soqlCount');
  }
  @IsTest static void entityNames() {
    Map<String, String> got = new Map<String, String>();
    for (C14Rule__mdt r : [SELECT DeveloperName, Object__r.QualifiedApiName, Field__r.QualifiedApiName FROM C14Rule__mdt]) {
      got.put(r.DeveloperName, r.Object__r.QualifiedApiName + '|' + r.Field__r.QualifiedApiName);
    }
    String described = C14Delivery__c.SObjectType.getDescribe().getName() + '|' + C14Delivery__c.Status__c.getDescribe().getName();
    System.assertEquals(2, got.size());
    System.assertEquals('gladens004__C14Delivery__c|gladens004__Status__c', described);
    System.assertEquals('gladens004__C14Delivery__c|gladens004__Status__c', got.get('Plain'));
    System.assertEquals('gladens004__C14Delivery__c|gladens004__Status__c', got.get('Prefixed'));
  }
}
