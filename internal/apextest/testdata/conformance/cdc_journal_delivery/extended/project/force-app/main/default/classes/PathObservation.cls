public class PathObservation {
 public static Integer afterInserts=0, afterUpdates=0;
 public static Boolean afterOwnerQueue;
 public class MakePath implements Queueable {public void execute(QueueableContext context) {insert new PathOracle__c(Name='queued',Mode__c='plain',Value__c='queued');}}
 public static String ownerKind(Id owner) {if(String.valueOf(owner).startsWith('00G')) return [SELECT DeveloperName FROM Group WHERE Id=:owner].DeveloperName;return 'user';}
 public static Map<String,Object> duplicateGraph(Datacloud.DuplicateResult result) {
  List<Object> matches=new List<Object>();
  for(Datacloud.MatchResult match:result.getMatchResults()) {
   List<Object> records=new List<Object>();for(Datacloud.MatchRecord record:match.getMatchRecords()) records.add(new Map<String,Object>{'confidence'=>record.getMatchConfidence(),'recordType'=>String.valueOf(record.getRecord().getSObjectType())});
   matches.add(new Map<String,Object>{'entity'=>match.getEntityType(),'success'=>match.isSuccess(),'records'=>records});
  }
  return new Map<String,Object>{'rule'=>result.getDuplicateRule(),'allowSave'=>result.isAllowSave(),'matches'=>matches};
 }
 public static void saveObservation(Map<String,Object> out,Database.SaveResult result) {
  out.put('success',result.isSuccess());List<Object> errors=new List<Object>();
  for(Database.Error failure:result.getErrors()) {
   Map<String,Object> error=new Map<String,Object>{'status'=>String.valueOf(failure.getStatusCode()),'message'=>failure.getMessage(),'duplicateError'=>failure instanceof Database.DuplicateError};
   if(failure instanceof Database.DuplicateError) error.put('duplicate',duplicateGraph(((Database.DuplicateError)failure).getDuplicateResult()));errors.add(error);
  }out.put('errors',errors);
 }
 public static void cdcLog(SObject event,EventBus.ChangeEventHeader header) {
  List<String> changed=new List<String>(),nulled=new List<String>();
  for(String field:header.changedFields) if(field=='Name'||field=='Description'||field=='Value__c'||field=='Other__c') changed.add(field);
  for(String field:header.nulledFields) if(field=='Name'||field=='Description'||field=='Value__c'||field=='Other__c') nulled.add(field);
  changed.sort();nulled.sort();
  Map<String,Object> row=new Map<String,Object>{'entity'=>header.entityName,'type'=>header.changeType,'recordCount'=>header.recordIds.size(),'changed'=>changed,'nulled'=>nulled,'commitUserIsSubscriber'=>header.commitUser==UserInfo.getUserId(),'automatedSubscriber'=>UserInfo.getName()=='Automated Process','commitUser'=>header.commitUser,'transactionKey'=>header.transactionKey,'sequence'=>header.sequenceNumber};
  if(header.entityName=='Account') {row.put('name',event.get('Name'));row.put('description',event.get('Description'));}
  if(header.entityName=='ChangeOracle__c') {row.put('value',event.get('Value__c'));row.put('other',event.get('Other__c'));}
  insert new OracleLog__c(Name='cdc',Data__c=JSON.serialize(row));
 }
 public static void collectCDC(Map<String,Object> out) {
  List<Object> events=new List<Object>();String firstKey;Long lastSequence;Boolean sameKey=true,increasing=true;
  for(OracleLog__c log:[SELECT Data__c FROM OracleLog__c WHERE Name='cdc' ORDER BY CreatedDate,Id]) {
   Map<String,Object> event=(Map<String,Object>)JSON.deserializeUntyped(log.Data__c);event.put('commitUserIsCaller',event.remove('commitUser')==UserInfo.getUserId());String txn=(String)event.remove('transactionKey');Long seq=Long.valueOf(String.valueOf(event.remove('sequence')));
   if(firstKey==null) firstKey=txn;else sameKey=sameKey&&(txn==firstKey);if(lastSequence!=null) increasing=increasing&&(seq>lastSequence);lastSequence=seq;events.add(event);
  }out.put('events',events);out.put('sameTransaction',sameKey);out.put('increasingSequence',increasing);
 }
 public static Map<String,Object> observe(String key) {
  Map<String,Object> out=new Map<String,Object>{'exception'=>null};
  try {
   if(key.startsWith('SP')) {
    String mode=key=='SP05'?'reject':key=='SP06'?'fault':key=='SP08'?'past':key=='SP09'?'future':'plain';
    PathOracle__c row=new PathOracle__c(Name=key,Mode__c=mode,Value__c='initial',Due__c=System.now().addDays(-1));
    Savepoint point;
    if(key=='SP03') {insert row;Test.startTest();}else {Test.startTest();if(key=='SP07') point=Database.setSavepoint();if(key=='SP10') System.enqueueJob(new MakePath());else insert row;}
    if(key=='SP04') {row.Value__c='updated';update row;}if(key=='SP05') {row.Mode__c='update';update row;}if(key=='SP07') Database.rollback(point);
    out.put('beforeCopied',[SELECT count() FROM PathOracle__c WHERE Copied__c!=null]);Test.stopTest();
    List<Object> values=new List<Object>();for(PathOracle__c record:[SELECT Value__c,Copied__c FROM PathOracle__c ORDER BY Name]) values.add(new Map<String,Object>{'value'=>record.Value__c,'copied'=>record.Copied__c});out.put('records',values);
    if(key=='SP11') {out.put('jobs',[SELECT count() FROM AsyncApexJob]);out.put('interviews',[SELECT count() FROM FlowInterview]);}
   } else if(key.startsWith('CDC')) {
    if(key=='CDC03') {insert new Contact(LastName='selected');Test.getEventBus().deliver();collectCDC(out);return out;}
    if(key=='CDC17') {out.put('setupRecords',[SELECT count() FROM Account WHERE Name='oracle-setup']);Test.enableChangeDataCapture();Test.getEventBus().deliver();collectCDC(out);return out;}
    Account account=new Account(Name='initial',Description='kept');if(key=='CDC04'||key=='CDC05'||key=='CDC06'||key=='CDC07'||key=='CDC12') insert account;Test.enableChangeDataCapture();
    if(key=='CDC04'||key=='CDC05'||key=='CDC06'||key=='CDC07') {Test.getEventBus().deliver();delete [SELECT Id FROM OracleLog__c WHERE Name='cdc'];}
    if(key=='CDC10') {insert new Account(Name='before-start');Test.startTest();}
    if(key=='CDC11') {Savepoint point=Database.setSavepoint();insert account;Database.rollback(point);}
    else if(key=='CDC04') {account.Name='changed';update account;}else if(key=='CDC05') {account.Description=null;update account;}else if(key=='CDC06') delete account;else if(key=='CDC07') {delete account;Test.getEventBus().deliver();delete [SELECT Id FROM OracleLog__c WHERE Name='cdc'];undelete account;}
    else if(key=='CDC08') insert new List<Account>{new Account(Name='same'),new Account(Name='same')};else if(key=='CDC09') {insert account;account.Name='changed';update account;}
    else if(key=='CDC12') {}else if(key=='CDC14'||key=='CDC15') insert new ChangeOracle__c(Name=key=='CDC15'?'flow':'custom',Value__c='original',Other__c='kept');else insert account;
    out.put('before',[SELECT count() FROM OracleLog__c WHERE Name='cdc']);if(key=='CDC10') Test.stopTest();else Test.getEventBus().deliver();collectCDC(out);
   } else if(key.startsWith('DR')) {
    if(key=='DR12') {insert new Contact(LastName='Synthetic',FirstName='Exact',Email='owned@example.invalid');saveObservation(out,Database.insert(new Contact(LastName='Synthetic',FirstName='Exact',Email='owned@example.invalid'),false));return out;}
    String token=key=='DR09'?null:'same-key';insert new MatchOracle__c(Name='seed',Key__c=token,Mode__c='seed');
    String mode=key=='DR03'||key=='DR04'?'alert':key=='DR06'?'report':key=='DR09'?'blank':key=='DR11'?'inactive':key=='DR13'?'before':key=='DR16'?'multi':'block';
    MatchOracle__c row=new MatchOracle__c(Name='candidate',Key__c=key=='DR10'?'SAME-KEY':key=='DR13'?'different':token,Mode__c=mode,Enabled__c=key!='DR15');
    if(key=='DR02') {try {insert row;out.put('saved',true);}catch(DmlException failure) {out.put('exception',failure.getTypeName());out.put('status',String.valueOf(failure.getDmlType(0)));out.put('mentionsDuplicate',failure.getMessage().contains('Owned exact-key duplicate'));}}
    else if(key=='DR07') {List<Database.SaveResult> results=Database.insert(new List<MatchOracle__c>{new MatchOracle__c(Name='one',Key__c='new-key',Mode__c='block',Enabled__c=true),new MatchOracle__c(Name='two',Key__c='new-key',Mode__c='block',Enabled__c=true)},false);List<Object> statuses=new List<Object>();for(Database.SaveResult result:results) {Map<String,Object> status=new Map<String,Object>();saveObservation(status,result);statuses.add(status);}out.put('results',statuses);}
    else if(key=='DR14') {List<Datacloud.FindDuplicatesResult> found=Datacloud.FindDuplicates.findDuplicates(new List<SObject>{row});List<Object> graphs=new List<Object>();for(Datacloud.FindDuplicatesResult result:found) {List<Object> duplicates=new List<Object>();for(Datacloud.DuplicateResult duplicate:result.getDuplicateResults()) duplicates.add(duplicateGraph(duplicate));graphs.add(new Map<String,Object>{'success'=>result.isSuccess(),'duplicates'=>duplicates});}out.put('results',graphs);
     delete [SELECT Id FROM MatchOracle__c];insert row;insert new MatchOracle__c(Name='id-match',Key__c=token,Mode__c='seed');
     List<Object> idGraphs=new List<Object>();for(Datacloud.FindDuplicatesResult result:Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{row.Id})) {List<Object> duplicates=new List<Object>();for(Datacloud.DuplicateResult duplicate:result.getDuplicateResults()) duplicates.add(duplicateGraph(duplicate));idGraphs.add(new Map<String,Object>{'success'=>result.isSuccess(),'duplicates'=>duplicates});}out.put('byIds',idGraphs);}
    else if(key=='DR13') {saveObservation(out,Database.insert(row,false));MatchOracle__c invalid=new MatchOracle__c(Name='invalid',Key__c=token,Mode__c='block',Enabled__c=true);Map<String,Object> validation=new Map<String,Object>();saveObservation(validation,Database.insert(invalid,false));out.put('validation',validation);}
    else {Database.DMLOptions options=new Database.DMLOptions();options.DuplicateRuleHeader.allowSave=key=='DR04';saveObservation(out,Database.insert(row,options));}
    out.put('count',[SELECT count() FROM MatchOracle__c]);if(key=='DR06') {out.put('sets',[SELECT count() FROM DuplicateRecordSet]);out.put('items',[SELECT count() FROM DuplicateRecordItem]);}
   } else if(key.startsWith('AR')) {
    afterInserts=0;afterUpdates=0;afterOwnerQueue=null;if(key=='AR15') {out.put('activeLeadRules',[SELECT count() FROM AssignmentRule WHERE SobjectType='Lead' AND Active=true]);return out;}
    Database.DMLOptions options=new Database.DMLOptions();options.assignmentRuleHeader.useDefaultRule=true;
    if(key=='AR04'||key=='AR13') {options.assignmentRuleHeader.assignmentRuleId=[SELECT Id FROM AssignmentRule WHERE Name='OracleLeadInactive' AND SobjectType='Lead' LIMIT 1].Id;if(key=='AR04') options.assignmentRuleHeader.useDefaultRule=null;}
    if(key=='AR05') {options.assignmentRuleHeader.useDefaultRule=null;options.assignmentRuleHeader.assignmentRuleId=[SELECT Id FROM AssignmentRule WHERE SobjectType='Case' AND Active=true LIMIT 1].Id;}if(key=='AR14') options.assignmentRuleHeader.useDefaultRule=false;
    if(key=='AR12') {List<Object> cases=new List<Object>();for(String subject:new List<String>{'match','miss'}) {Case row=new Case(Subject=subject,Status='New',Origin='Web');Database.SaveResult result=Database.insert(row,options);Map<String,Object> status=new Map<String,Object>();saveObservation(status,result);if(result.isSuccess()) {row=[SELECT OwnerId FROM Case WHERE Id=:result.Id];status.put('owner',ownerKind(row.OwnerId));}cases.add(status);}out.put('cases',cases);return out;}
    Lead lead=new Lead(LastName='Synthetic',Company=key=='AR07'?'multi':key=='AR08'?'formula':key=='AR09'?'miss':'match');Database.SaveResult result;
    if(key=='AR03') {lead.setOptions(options);insert lead;out.put('owner',ownerKind([SELECT OwnerId FROM Lead WHERE Id=:lead.Id].OwnerId));}
    else if(key=='AR09') {insert lead;lead.Company='match';result=Database.update(lead,options);}else result=Database.insert(lead,options);
    if(result!=null) {saveObservation(out,result);if(result.isSuccess()) out.put('owner',ownerKind([SELECT OwnerId FROM Lead WHERE Id=:result.Id].OwnerId));}
    if(key=='AR08') {Lead booleanLead=new Lead(LastName='Boolean',Company='multi');Database.SaveResult second=Database.insert(booleanLead,options);out.put('booleanOwner',ownerKind([SELECT OwnerId FROM Lead WHERE Id=:second.Id].OwnerId));}
    if(key=='AR10') {out.put('afterInsertOwnerQueue',afterOwnerQueue);out.put('afterInserts',afterInserts);out.put('afterUpdates',afterUpdates);}
    if(key=='AR11') {List<Object> owners=new List<Object>();for(OracleLog__c log:[SELECT Data__c FROM OracleLog__c WHERE Name='lead-owner']) owners.add(log.Data__c);out.put('flowOwners',owners);}
   }
  } catch(Exception failure) {out.put('exception',failure.getTypeName());}return out;
 }
 public static void emit(String key) {System.assert(false,'PATH_OBS|'+key+'|'+JSON.serialize(observe(key)));}
}
