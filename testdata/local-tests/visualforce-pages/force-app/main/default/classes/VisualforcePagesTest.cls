@isTest
private class VisualforcePagesTest {
  @isTest static void pageTokensAndPageReferenceStateAreRegistered() {
    PageReference review = Page.OrderReview;
    System.assertEquals('/apex/OrderReview', review.getUrl());

    Test.setCurrentPageReference(Page.OrderList);
    ApexPages.currentPage().getParameters().put('id', '001000000000101AAA');

    System.assertEquals('/apex/OrderList', System.currentPageReference().getUrl());
    System.assertEquals('001000000000101AAA', ApexPages.currentPage().getParameters().get('id'));
  }

  @isTest static void standardControllerExtensionSavesAndKeepsPageState() {
    Account account = new Account(Name = 'Owned Fixture');
    ApexPages.StandardController controller = new ApexPages.StandardController(account);
    OrderReviewExtension extension = new OrderReviewExtension(controller);

    System.assertEquals('Owned Fixture review', extension.getTitle());
    PageReference saved = extension.saveAndReturn();

    System.assert(account.Id != null);
    System.assertEquals('/' + account.Id + '?source=visualforce-pages', saved.getUrl());
    System.assertEquals('visualforce-pages', saved.getParameters().get('source'));
  }

  @isTest static void standardSetControllerAndMessagesResetBetweenMethods() {
    System.assert(!ApexPages.hasMessages());
    ApexPages.StandardSetController controller = new ApexPages.StandardSetController(new List<Account>{
      new Account(Name = 'First'),
      new Account(Name = 'Second'),
      new Account(Name = 'Third')
    });
    controller.setPageSize(2);
    System.assert(controller.getHasNext());
    controller.next();
    System.assertEquals(2, controller.getPageNumber());
    System.assert(controller.getHasPrevious());

    ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.CONFIRM, 'Paged'));
    System.assert(ApexPages.hasMessages());
  }
}
