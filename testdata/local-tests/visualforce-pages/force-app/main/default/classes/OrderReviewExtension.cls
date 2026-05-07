public class OrderReviewExtension {
  private ApexPages.StandardController controller;

  public OrderReviewExtension(ApexPages.StandardController controller) {
    this.controller = controller;
  }

  public String getTitle() {
    Account account = (Account) controller.getRecord();
    return account.Name + ' review';
  }

  public PageReference saveAndReturn() {
    PageReference saved = controller.save();
    saved.getParameters().put('source', 'visualforce-pages');
    return saved;
  }
}
