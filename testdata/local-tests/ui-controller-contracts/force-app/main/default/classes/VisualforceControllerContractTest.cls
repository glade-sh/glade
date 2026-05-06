@isTest
private class VisualforceControllerContractTest {
  @isTest static void resolvesPageTokenAndExtensionConstructor() {
    Account account = new Account(Name = 'Acme');
    ApexPages.StandardController controller = new ApexPages.StandardController(account);
    AccountViewExtension extension = new AccountViewExtension(controller);
    System.assertEquals('Acme', extension.name);

    Test.setCurrentPage(Page.AccountView);
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    System.assertEquals('/apex/AccountView', ApexPages.currentPage().getUrl());
    System.assertEquals('001000000000001AAA', ApexPages.currentPage().getParameters().get('id'));
  }

  @isTest static void pageStateStartsClean() {
    System.assert(!ApexPages.hasMessages());
    System.assertEquals(null, ApexPages.currentPage().getParameters().get('id'));
  }
}
