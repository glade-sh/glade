public class RelationshipRefresh {
 public static void apply(List<SObject> items, Map<Id,SObject> replacements) {
  Map<String,Schema.SObjectField> fields=items[0].getSObjectType().getDescribe().fields.getMap();
  for(Integer i=0;i<items.size();i++) {
   SObject original=items[i], expanded=replacements.get(original.Id);
   Map<String,Object> values=expanded.getPopulatedFieldsAsMap();
   for(String name:values.keySet()) {
    Schema.DescribeFieldResult column=fields.get(name)?.getDescribe();
    if(column?.getReferenceTo().isEmpty()!=false || column?.getName()=='Id') continue;
    try {
     SObject parent=original.getSObject(column.getRelationshipName());
     if(parent==null) original.putSObject(column.getRelationshipName(),expanded.getSObject(column.getRelationshipName()));
     else parent.put(name,values.get(name));
    } catch(SObjectException e) {
     String relationship=column.getRelationshipName();
     if(values.containsKey(relationship)) {
      Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(original));
      payload.put(relationship,expanded.getSObject(relationship));
      items.set(i,(SObject)JSON.deserialize(JSON.serialize(payload),SObject.class));
     }
    }
   }
  }
 }
}
