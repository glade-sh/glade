public class SamCOm67b extends SamPOm66 {
  public String cc() { return 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]; }
}
