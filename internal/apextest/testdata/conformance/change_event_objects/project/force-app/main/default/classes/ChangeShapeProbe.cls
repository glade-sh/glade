public class ChangeShapeProbe {
 public static Map<String,Object> observe(String objectName) {
  SObject event=(SObject)Type.forName(objectName).newInstance();
  Map<String,Schema.SObjectField> fields=event.getSObjectType().getDescribe().fields.getMap();
  Map<String,Object> out=new Map<String,Object>{'name'=>event.getSObjectType().getDescribe().getName()};
  for(String field:new List<String>{'Name','Phone','Id','CreatedDate','OwnerId','Marker__c','ChangeEventHeader','ReplayId','Missing__c'}) {
   out.put(field,fields.containsKey(field));
  }
  return out;
 }
 public static void emit(String method,String objectName) {
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observe(objectName)));
 }
}
