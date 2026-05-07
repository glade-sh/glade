@isTest
private class PresentationMetadataTest {
  @isTest static void describesLocalTabs() {
    List<Object> tabSets = Schema.describeTabs();
    System.assertEquals(1, tabSets.size());
    Object tabSet = tabSets.get(0);
    System.assertEquals('All Tabs', tabSet.getLabel());
    List<Object> tabs = tabSet.getTabs();
    System.assertEquals(1, tabs.size());
    Object tab = tabs.get(0);
    System.assertEquals('Widget__c', tab.getName());
    System.assertEquals('Widgets', tab.getLabel());
    System.assertEquals('Widget__c', tab.getSObjectName());
    System.assert(tab.isCustom());
  }
}
