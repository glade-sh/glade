public class LegacyFlowAction {
  public static Integer Calls = 0;

  @InvocableMethod
  public static void run(List<Legacy_Work_Order__c> records) {
    Calls++;
  }
}
