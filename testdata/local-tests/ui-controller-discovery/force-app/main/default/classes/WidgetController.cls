public class WidgetController {
  @AuraEnabled(cacheable=true)
  public static String getWidget(String accountId) {
    return 'widget:' + accountId;
  }

  @AuraEnabled
  public static void saveWidget(String accountId) {
  }
}
