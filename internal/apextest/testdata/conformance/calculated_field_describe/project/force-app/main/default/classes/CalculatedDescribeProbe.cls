public class CalculatedDescribeProbe {
 public static String sample() {
  Map<String,Object> rows = new Map<String,Object>();
  Schema.SObjectType objectType = Schema.getGlobalDescribe().get('WorkOrder');
  rows.put('objectPresent', objectType != null);
  if (objectType == null) return JSON.serialize(rows);
  Map<String,Schema.SObjectField> fields = objectType.getDescribe().fields.getMap();
  for (String name : new List<String>{'LineItemCount','Subtotal','TotalPrice','WorkOrderNumber','Subject'}) {
   Map<String,Object> item = new Map<String,Object>{'present' => fields.containsKey(name)};
   if (fields.containsKey(name)) {
    Schema.DescribeFieldResult description = fields.get(name).getDescribe();
    item.put('calculated', description.isCalculated());
    item.put('displayType', String.valueOf(description.getType()));
   }
   rows.put(name, item);
  }
  return JSON.serialize(rows);
 }
}
