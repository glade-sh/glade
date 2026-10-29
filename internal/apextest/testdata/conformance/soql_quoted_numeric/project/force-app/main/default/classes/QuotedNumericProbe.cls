public class QuotedNumericProbe {
 public static Object observe(String shape,String route) {
  String predicate;
  if(shape=='name_eq') predicate='Name = 3';
  if(shape=='name_in') predicate='Name IN (1,2)';
  if(shape=='name_decimal') predicate='Name = 3.5';
  if(shape=='name_quoted_eq') predicate='Name = \'3\'';
  if(shape=='name_quoted_in') predicate='Name IN (\'3\',\'2\')';
  if(shape=='name_null') predicate='Name = null';
  if(shape=='picklist_eq') predicate='Industry = 3';
  if(shape=='picklist_in') predicate='Industry IN (1,2)';
  if(shape=='picklist_quoted') predicate='Industry = \'Technology\'';
  if(shape=='textarea_eq') predicate='Description = 3';
  if(shape=='textarea_quoted') predicate='Description = \'3\'';
  if(shape=='short_textarea_eq') predicate='Note__c = 3';
  if(shape=='short_textarea_quoted') predicate='Note__c = \'3\'';
  if(shape=='id_eq') predicate='Id = 3';
  if(shape=='id_in') predicate='Id IN (1,2)';
  if(shape=='id_null') predicate='Id = null';
  try {
   if(route=='sosl') { List<List<SObject>> groups=Search.query('FIND \'quotednumericnomatch\' IN ALL FIELDS RETURNING Account(Id WHERE '+predicate+')'); return groups[0].size(); }
   insert new Account(Name='3',Industry='Technology',Description='3',Note__c='3');
   String q='SELECT Id FROM Account WHERE '+predicate;
   if(route=='locator') { Database.QueryLocator loc=Database.getQueryLocator(q); return 'locator'; }
   List<SObject> rows;
   if(route=='queryWithBinds') rows=Database.queryWithBinds(q,new Map<String,Object>(),AccessLevel.SYSTEM_MODE);
   else rows=Database.query(q);
   return !rows.isEmpty();
  } catch(Exception e) { return new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()}; }
 }
}
