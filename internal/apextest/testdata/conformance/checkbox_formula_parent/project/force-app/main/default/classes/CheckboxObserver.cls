public class CheckboxObserver {
 public static Map<String,Object> values = new Map<String,Object>();
 public static void save(String route, Item__c row) {
  values.put(route + ':' + row.Name, new Map<String,Object>{'flag'=>row.IsHeld__c,'text'=>row.TextFormula__c});
 }
}
