@isTest
private class WorkflowTest {
  @isTest static void workflowFieldUpdateRunsDuringDml() {
    Widget__c widget = new Widget__c(Name = 'Acme');
    insert widget;

    Widget__c reloaded = [SELECT Name, Status__c FROM Widget__c WHERE Id = :widget.Id LIMIT 1];
    System.assertEquals('Active', reloaded.Status__c);
    System.assertEquals(1, Limits.getEmailInvocations());

    reloaded.Name = 'Other';
    reloaded.Status__c = null;
    update reloaded;

    Widget__c miss = [SELECT Status__c FROM Widget__c WHERE Id = :reloaded.Id LIMIT 1];
    System.assertEquals(null, miss.Status__c);
    System.assertEquals(1, Limits.getEmailInvocations());
  }
}
