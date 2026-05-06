public class WidgetFlowAction {
  public static Integer Calls = 0;

  @InvocableMethod
  public static void run(List<Widget__c> widgets) {
    Calls = Calls + widgets.size();
  }
}
