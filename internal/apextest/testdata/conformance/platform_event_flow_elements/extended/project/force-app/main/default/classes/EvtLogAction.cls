public class EvtLogAction {
 @InvocableMethod public static void record(List<String> keys) { insert new EvtLog__c(Name=keys[0],Phase__c='apexfault'); }
}
