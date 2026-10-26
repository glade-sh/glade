public class ElementAction {
 @InvocableMethod public static void capture(List<ElementSignal__e> signals) {
  for(ElementSignal__e signal:signals) insert new ElementLog__c(Name=signal.Key__c,Phase__c='action',Actor__c=UserInfo.getUserId());
 }
}
