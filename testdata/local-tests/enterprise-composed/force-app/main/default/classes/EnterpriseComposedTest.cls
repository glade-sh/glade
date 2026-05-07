@isTest
private class EnterpriseComposedTest {
  @isTest static void visualforceSaveRunsDeclarativeMetadataAndPresentationContracts() {
    System.assertEquals('Escalated Legacy', Label.LegacyStatus);
    Enterprise_Feature__mdt cfg = Enterprise_Feature__mdt.getInstance('Default');
    System.assertEquals(true, cfg.Enabled__c);
    System.assertEquals('legacy-default', cfg.Route__c);

    Legacy_Work_Order__c work = new Legacy_Work_Order__c(Name = 'Legacy Acme', Priority__c = 9);
    ApexPages.StandardController controller = new ApexPages.StandardController(work);
    LegacyWorkOrderReviewExtension extension = new LegacyWorkOrderReviewExtension(controller);
    Test.setCurrentPage(Page.LegacyWorkOrderReview);
    ApexPages.currentPage().getParameters().put('mode', 'review');

    System.assertEquals('Escalated Legacy: Legacy Acme', extension.getTitle());
    PageReference saved = extension.saveAndReturn();

    System.assert(work.Id != null);
    System.assertEquals('/' + work.Id, saved.getUrl());
    System.assertEquals('enterprise-composed', saved.getParameters().get('source'));
    System.assertEquals('review', ApexPages.currentPage().getParameters().get('mode'));

    List<Legacy_Work_Order__c> rows = Database.query('SELECT Status__c, Workflow_Marker__c, Flow_Marker__c, Active__c, Label_Marker__c FROM Legacy_Work_Order__c WHERE Id = :work.Id');
    Legacy_Work_Order__c reloaded = rows.get(0);
    System.assertEquals('Workflow Active', reloaded.Workflow_Marker__c);
    System.assertEquals('Flow-Legacy Acme', reloaded.Flow_Marker__c);
    System.assertEquals(true, reloaded.Active__c);
    System.assertEquals('Escalated Legacy', reloaded.Label_Marker__c);
    System.assertEquals(1, LegacyFlowAction.Calls);

    List<Object> tabSets = Schema.describeTabs();
    System.assertEquals(1, tabSets.size());
    Object tabSet = tabSets.get(0);
    List<Object> tabs = tabSet.getTabs();
    System.assertEquals(1, tabs.size());
    Object tab = tabs.get(0);
    System.assertEquals('Legacy_Work_Order__c', tab.getSObjectName());
    System.assertEquals('Legacy Work Orders', tab.getLabel());

    Metadata.DeployContainer container = new Metadata.DeployContainer();
    Metadata.CustomMetadata deployed = new Metadata.CustomMetadata();
    deployed.fullName = 'Enterprise_Feature.Deployed';
    deployed.label = 'Deployed Enterprise Feature';
    Metadata.CustomMetadataValue enabled = new Metadata.CustomMetadataValue();
    enabled.field = 'Enabled__c';
    enabled.value = true;
    deployed.values.add(enabled);
    Metadata.CustomMetadataValue route = new Metadata.CustomMetadataValue();
    route.field = 'Route__c';
    route.value = 'deployed-route';
    deployed.values.add(route);
    container.addMetadata(deployed);

    String deploymentId = Metadata.Operations.enqueueDeployment(container, null);
    System.assertEquals('0Af000000000001', deploymentId);
    Enterprise_Feature__mdt deployedCfg = Enterprise_Feature__mdt.getInstance('Deployed');
    System.assertEquals(true, deployedCfg.Enabled__c);
    System.assertEquals('deployed-route', deployedCfg.Route__c);

    List<Metadata.CustomMetadata> records = Metadata.Operations.retrieve(
      Metadata.MetadataType.CustomMetadata,
      new List<String>{'Enterprise_Feature.Default', 'Enterprise_Feature.Deployed'}
    );
    System.assertEquals(2, records.size());
    System.assertEquals('Enterprise_Feature.Default', records[0].fullName);
    System.assertEquals('Enterprise_Feature.Deployed', records[1].fullName);
  }

  @isTest static void pageAndStaticStateResetBetweenMethods() {
    System.assert(!ApexPages.hasMessages());
    System.assertEquals(null, ApexPages.currentPage().getParameters().get('mode'));
    System.assertEquals(0, LegacyFlowAction.Calls);
  }
}
