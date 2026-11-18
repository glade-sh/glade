public class KeHolder {
 public static Set<String> fields() { return Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).fields.getMap().keySet(); }
 public static String run() {
  Set<String> k = fields(); Integer n = k.size();
  k.addAll(new Map<String,String>().values());
  Boolean r = k.addAll(new List<String>());
  return r + '|' + (k.size() - n);
 }
}