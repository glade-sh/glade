public class DmlPolicyProbe {
 public static String duplicateRow(String key,String policy,String operation,String route,String allowSave) {
  String email=key+'@example.invalid';
  insert new Contact(LastName=policy,Email=email);
  Contact row=new Contact(LastName=policy,Email=operation=='update'?'other-'+email:email);
  if(operation=='update') insert row;
  row.Email=email;
  Database.DMLOptions options=new Database.DMLOptions();
  if(allowSave!='null') options.DuplicateRuleHeader.allowSave=allowSave=='true';
  options.DuplicateRuleHeader.runAsCurrentUser=true;
  String outcome='ok';
  try {
   if(route=='keyword') { if(operation=='insert') insert row; else update row; }
   else if(route=='record') { row.setOptions(options); if(operation=='insert') insert row; else update row; }
   else {
    Database.SaveResult result;
    if(route=='database') { if(operation=='insert') result=Database.insert(row,false); else result=Database.update(row,false); }
    else { if(operation=='insert') result=Database.insert(row,options); else result=Database.update(row,options); }
    if(!result.isSuccess()) outcome=String.valueOf(result.getErrors()[0].getStatusCode());
   }
  } catch(DmlException error) { outcome=String.valueOf(error.getDmlStatusCode(0)); }
  List<Contact> rows=[SELECT Email FROM Contact WHERE Id=:row.Id];
  return outcome+'|'+(rows.size()==1 && rows[0].Email==email);
 }
 public static String assignmentRow(String objectName,String operation,String route,String selector) {
  SObject row=objectName=='Case'?(SObject)new Case(Subject='Before',Status='New',Origin='Phone'):(SObject)new Lead(LastName='Synthetic',Company='Before');
  if(operation=='update') insert row;
  row.put(objectName=='Case'?'Subject':'Company','Route');
  Database.DMLOptions options=new Database.DMLOptions();
  if(selector=='default') options.assignmentRuleHeader.useDefaultRule=true;
  if(selector=='false') options.assignmentRuleHeader.useDefaultRule=false;
  if(selector=='id') options.assignmentRuleHeader.assignmentRuleId=[SELECT Id FROM AssignmentRule WHERE Name='SyntheticRouting' AND SobjectType=:objectName LIMIT 1].Id;
  if(route=='record') {row.setOptions(options);if(operation=='insert')insert row;else update row;}
  else if(route=='keyword') {if(operation=='insert')insert row;else update row;}
  else {Database.SaveResult result;if(operation=='insert')result=Database.insert(row,options);else result=Database.update(row,options);if(!result.isSuccess())return String.valueOf(result.getErrors()[0].getStatusCode());}
  Id rowId=(Id)row.get('Id');
  SObject stored=Database.query('SELECT OwnerId FROM '+objectName+' WHERE Id=:rowId');
  Id queueId=[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;
  return (stored.get('OwnerId')==queueId)+'|'+(stored.get('OwnerId')==UserInfo.getUserId());
 }
 public static String calloutRow(String operation) {
  if(operation=='insert') insert new Account(Name='Pending');
  else if(operation=='database') Database.insert(new Account(Name='Pending'),false);
  else if(operation=='failed') Database.insert(new Account(),false);
  else if(operation=='update' || operation=='delete' || operation=='rollback') {
   Account row=new Account(Name='Pending');insert row;
   if(operation=='update') {row.Name='Changed';update row;}
   if(operation=='delete') delete row;
   if(operation=='rollback') {Savepoint point=Database.setSavepoint();insert new Account(Name='Rolled back');Database.rollback(point);}
  }
  HttpRequest request=new HttpRequest();request.setEndpoint('https://example.com');request.setMethod('GET');
  try {return 'ok|'+new Http().send(request).getStatusCode();}
  catch(Exception error) {return error.getTypeName()+'|'+error.getMessage();}
 }
}
