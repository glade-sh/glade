public class QueryShapeProbe {
 public static Object observe(String shape, String route) {
  if (!shape.startsWith('count_')) insert new Account(Name='Shape Alpha', Phone='5550100', Industry='Technology');
  String q;
  if(shape=='count_missing') q='SELECT COUNT() FROM Account WHERE Bogus__c = \'x\'';
  if(shape=='count_parent_missing') q='SELECT COUNT() FROM Contact WHERE Account.Bogus__c = \'x\'';
  if(shape=='count_bad_type') q='SELECT COUNT() FROM Account WHERE Name = 3';
  if(shape=='count_valid') q='SELECT COUNT() FROM Account WHERE Name = \'No synthetic match\'';
  if(shape=='count_empty') q='SELECT COUNT() FROM Account WHERE Name = null';
  if(shape=='group_hidden') q='SELECT COUNT(Id) total FROM Account WHERE Name = \'Shape Alpha\' GROUP BY Name';
  if(shape=='group_selected') q='SELECT Name, COUNT(Id) total FROM Account WHERE Name = \'Shape Alpha\' GROUP BY Name';
  if(shape=='group_other_hidden') q='SELECT COUNT(Id) total FROM Account WHERE Name = \'Shape Alpha\' GROUP BY Industry';
  if(shape=='group_alias') q='SELECT Name label, COUNT(Id) total FROM Account WHERE Name = \'Shape Alpha\' GROUP BY Name';
  if(shape=='group_only') q='SELECT Name FROM Account WHERE Name = \'Shape Alpha\' GROUP BY Name';
  if(shape=='order_name_id') q='SELECT Name, Id FROM Account WHERE Name = \'Shape Alpha\'';
  if(shape=='order_id_name') q='SELECT Id, Name FROM Account WHERE Name = \'Shape Alpha\'';
  if(shape=='order_three') q='SELECT Phone, Name, Id FROM Account WHERE Name = \'Shape Alpha\'';
  if(shape=='order_three_reverse') q='SELECT Id, Name, Phone FROM Account WHERE Name = \'Shape Alpha\'';
  if(shape=='order_null') q='SELECT Description, Name, Id FROM Account WHERE Name = \'Shape Alpha\'';
  try {
   if(route=='countQuery') return Database.countQuery(q);
   if(route=='locator') { Database.QueryLocator loc=Database.getQueryLocator(q); return 'locator'; }
   List<SObject> rows;
   if(route=='queryWithBinds') rows=Database.queryWithBinds(q,new Map<String,Object>(),AccessLevel.SYSTEM_MODE);
   else rows=Database.query(q);
   if(rows.isEmpty()) return new List<String>();
   return new List<String>(rows[0].getPopulatedFieldsAsMap().keySet());
  } catch(Exception e) { return new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()}; }
 }
}
