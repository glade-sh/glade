@isTest
private class FlowTest {
  @isTest static void recordTriggeredFlowFieldUpdateRunsDuringDml() {
    Widget__c widget = new Widget__c(Name = 'Acme', Score__c = 12);
    insert widget;

    Widget__c reloaded = [SELECT Name, Status__c, Score_Copy__c, Active__c FROM Widget__c WHERE Id = :widget.Id LIMIT 1];
    System.assertEquals('Flow-Acme', reloaded.Status__c);
    System.assert(reloaded.Score_Copy__c != null);
    System.assertEquals(true, reloaded.Active__c);
    System.assertEquals(1, WidgetFlowAction.Calls);

    reloaded.Name = 'Other';
    reloaded.Status__c = null;
    reloaded.Score_Copy__c = null;
    reloaded.Active__c = false;
    update reloaded;

    Widget__c miss = [SELECT Status__c, Score_Copy__c, Active__c FROM Widget__c WHERE Id = :reloaded.Id LIMIT 1];
    System.assertEquals(null, miss.Status__c);
    System.assertEquals(null, miss.Score_Copy__c);
    System.assertEquals(false, miss.Active__c);
    System.assertEquals(1, WidgetFlowAction.Calls);
  }
}
