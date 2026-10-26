public class DmlPolicyRouteProbe {
 public static Boolean beforeOwned=false;
 public static Boolean afterOwned=false;
 public static String nested='missing';
 public static List<String> ownerEvents=new List<String>();
 public static void observeCases(List<Case> rows,Boolean before,Boolean inserting) {
  for(Case row:rows) {
   if(row.Subject=='Route'){ownerEvents.add((before?'before':'after')+(inserting?'-insert:':'-update:')+(row.OwnerId==[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id));if(before)beforeOwned=row.OwnerId==[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;else afterOwned=row.OwnerId==[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;}
   if(!before && row.Subject=='NestedDuplicate') {
    try{insert new Contact(LastName='Block',Email='nested-route@example.invalid');nested='ok';}catch(DmlException error){nested=String.valueOf(error.getDmlStatusCode(0));}
   }
  }
 }
 public static String duplicateBatch(String key,String policy,String route,Boolean allowSave) {
  String email=key+'@example.invalid';insert new Contact(LastName=policy,Email=email);
  List<Contact> rows=new List<Contact>{new Contact(LastName=policy,Email='new-'+email),new Contact(LastName=policy,Email=email)};
  Database.DMLOptions options=new Database.DMLOptions();options.DuplicateRuleHeader.allowSave=allowSave;
  String status='';
  if(route=='keyword'){try{insert rows;status='ok';}catch(DmlException error){status=String.valueOf(error.getDmlStatusCode(0));}}
  else {List<Database.SaveResult> results=Database.insert(rows,options);for(Database.SaveResult result:results){if(status!='')status+=',';status+=result.isSuccess()?'ok':String.valueOf(result.getErrors()[0].getStatusCode());}}
  String other='new-'+email;
  return status+'|'+[SELECT COUNT() FROM Contact WHERE Email=:email OR Email=:other];
 }
 public static String duplicateUpsert(String key,String route) {
  String email=key+'@example.invalid';insert new Contact(LastName='Block',Email=email);
  Contact row=new Contact(LastName='Block',Email=email);
  String status='ok';
  if(route=='keyword'){try{upsert row;}catch(DmlException error){status=String.valueOf(error.getDmlStatusCode(0));}}
  else {Database.UpsertResult result=Database.upsert(row,false);if(!result.isSuccess())status=String.valueOf(result.getErrors()[0].getStatusCode());}
  return status+'|'+[SELECT COUNT() FROM Contact WHERE Email=:email];
 }
 public static String assignmentTrigger(String operation,String route) {
  beforeOwned=false;afterOwned=false;ownerEvents.clear();
  Case row=new Case(Subject='Before',Status='New',Origin='Phone');if(operation=='update')insert row;row.Subject='Route';
  Database.DMLOptions options=new Database.DMLOptions();options.assignmentRuleHeader.useDefaultRule=true;
  if(route=='record'){row.setOptions(options);if(operation=='insert')insert row;else update row;}
  else {if(operation=='insert')Database.insert(row,options);else Database.update(row,options);}
  Id queueId=[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;
  return beforeOwned+'|'+afterOwned+'|'+([SELECT OwnerId FROM Case WHERE Id=:row.Id].OwnerId==queueId)+'|'+String.join(ownerEvents,',');
 }
 public static String nestedDuplicate(){insert new Contact(LastName='Block',Email='nested-route@example.invalid');insert new Case(Subject='NestedDuplicate',Status='New',Origin='Phone');return nested;}
 public class CalloutWorker implements Queueable,Database.AllowsCallouts {public void execute(QueueableContext context){HttpRequest request=new HttpRequest();request.setEndpoint('https://example.com');request.setMethod('GET');System.assertEquals(204,new Http().send(request).getStatusCode());}}
 public static String asyncCallout(){if(!Test.isRunningTest())return 'test-only';Test.startTest();insert new Account(Name='Parent');System.enqueueJob(new CalloutWorker());Test.stopTest();return 'ok';}

 public static void observeLeads(List<Lead> rows,Boolean before,Boolean inserting){for(Lead row:rows){if(row.Company=='Route'){Boolean queued=row.OwnerId==[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;ownerEvents.add((before?'before':'after')+(inserting?'-insert:':'-update:')+queued);if(before)beforeOwned=queued;else afterOwned=queued;}}}
 public static String assignmentLeadTrigger(String operation,String route){beforeOwned=false;afterOwned=false;ownerEvents.clear();Lead row=new Lead(LastName='Synthetic',Company='Before');if(operation=='update')insert row;row.Company='Route';Database.DMLOptions options=new Database.DMLOptions();options.assignmentRuleHeader.useDefaultRule=true;if(route=='record'){row.setOptions(options);if(operation=='insert')insert row;else update row;}else{if(operation=='insert')Database.insert(row,options);else Database.update(row,options);}Id queueId=[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='SyntheticRouting' LIMIT 1].Id;return beforeOwned+'|'+afterOwned+'|'+([SELECT OwnerId FROM Lead WHERE Id=:row.Id].OwnerId==queueId)+'|'+String.join(ownerEvents,',');}
}
