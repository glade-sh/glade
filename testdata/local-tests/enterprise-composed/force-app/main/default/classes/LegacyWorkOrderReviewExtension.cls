public class LegacyWorkOrderReviewExtension {
  private ApexPages.StandardController controller;
  public Legacy_Work_Order__c SavedWork;

  public LegacyWorkOrderReviewExtension(ApexPages.StandardController controller) {
    this.controller = controller;
  }

  public String getTitle() {
    Legacy_Work_Order__c work = (Legacy_Work_Order__c) controller.getRecord();
    return Label.LegacyStatus + ': ' + work.Name;
  }

  public PageReference saveAndReturn() {
    Legacy_Work_Order__c work = (Legacy_Work_Order__c) controller.getRecord();
    work.Label_Marker__c = Label.LegacyStatus;
    PageReference saved = controller.save();
    SavedWork = (Legacy_Work_Order__c) controller.getRecord();
    saved.getParameters().put('source', 'enterprise-composed');
    return saved;
  }
}
