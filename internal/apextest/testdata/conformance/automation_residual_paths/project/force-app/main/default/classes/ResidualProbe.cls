@IsTest private class ResidualProbe {
 class Mock implements HttpCalloutMock { public HttpResponse respond(HttpRequest r){ HttpResponse s=new HttpResponse(); s.setStatusCode(200); s.setBody('ok'); return s; } }
 static void obs(String n, Map<String,Object> m){ System.assert(false,'OBSERVED|'+n+'|'+JSON.serialize(m)); }
 static Map<String,Object> res(Database.SaveResult r){
  Map<String,Object> m=new Map<String,Object>{'ok'=>r.isSuccess()};
  if(!r.isSuccess()){ Database.Error e=r.getErrors()[0]; m.put('code',String.valueOf(e.getStatusCode())); m.put('msg',e.getMessage()); m.put('dup',e instanceof Database.DuplicateError); }
  return m;
 }
 static Boolean marker(Id i){ return [SELECT Marker__c FROM Parcel__c WHERE Id=:i WITH SYSTEM_MODE].Marker__c; }
 static void seedDup(){ insert new Lead(LastName='Seed',Company='Seed',Email='dup@probe.test',Phone='5551'); }
 static Id queueId(){ return [SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='ResidualQueue'].Id; }
 static Database.DMLOptions opts(Boolean assign, Boolean allowSave){
  Database.DMLOptions o=new Database.DMLOptions();
  if(assign) o.assignmentRuleHeader.useDefaultRule=true;
  o.duplicateRuleHeader.allowSave=allowSave;
  return o;
 }
 @IsTest static void schedPast(){ Test.startTest(); Parcel__c p=new Parcel__c(Name='a',Tag__c='past',NextRun__c=System.now().addDays(-1)); Database.insert(p,AccessLevel.SYSTEM_MODE); Boolean b=marker(p.Id); Test.stopTest(); obs('schedPast',new Map<String,Object>{'before'=>b,'after'=>marker(p.Id)}); }
 @IsTest static void schedExec(){ Test.startTest(); Parcel__c p=new Parcel__c(Name='a',Tag__c='exec'); Database.insert(p,AccessLevel.SYSTEM_MODE); Boolean b=marker(p.Id); Test.stopTest(); obs('schedExec',new Map<String,Object>{'before'=>b,'after'=>marker(p.Id)}); }
 @IsTest static void schedUpdate(){ Test.startTest(); Parcel__c p=new Parcel__c(Name='a',Tag__c='upd',Qty__c=1); Database.insert(p,AccessLevel.SYSTEM_MODE); p.Qty__c=2; Database.update(p,AccessLevel.SYSTEM_MODE); Boolean b=marker(p.Id); Test.stopTest(); obs('schedUpdate',new Map<String,Object>{'before'=>b,'after'=>marker(p.Id)}); }
 @IsTest static void flowCallout(){
  Test.setMock(HttpCalloutMock.class,new Mock());
  Map<String,Object> m=new Map<String,Object>();
  try{ Database.insert(new Parcel__c(Name='a',Tag__c='callout'),AccessLevel.SYSTEM_MODE); m.put('thrown',false); }catch(DmlException e){ m.put('thrown',true); m.put('msg',e.getMessage().left(500)); }
  m.put('rows',[SELECT count() FROM Parcel__c WITH SYSTEM_MODE]);
  obs('flowCallout',m);
 }
 @IsTest static void flowCalloutAfterStart(){
  Test.setMock(HttpCalloutMock.class,new Mock());
  Test.startTest();
  Map<String,Object> m=new Map<String,Object>();
  try{ Database.insert(new Parcel__c(Name='a',Tag__c='callout'),AccessLevel.SYSTEM_MODE); m.put('thrown',false); }catch(DmlException e){ m.put('thrown',true); m.put('msg',e.getMessage().left(500)); }
  Test.stopTest();
  m.put('rows',[SELECT count() FROM Parcel__c WITH SYSTEM_MODE]);
  obs('flowCalloutAfterStart',m);
 }
 @IsTest static void dupBeforeFlow(){ System.assert(false,'OBSERVED|dupBeforeFlow|'+ResidualRun.run('dupBeforeFlow')); }
 @IsTest static void dupAfterFlow(){ System.assert(false,'OBSERVED|dupAfterFlow|'+ResidualRun.run('dupAfterFlow')); }
 @IsTest static void dupTrigger(){ System.assert(false,'OBSERVED|dupTrigger|'+ResidualRun.run('dupTrigger')); }
 @IsTest static void dupAssignBlock(){ System.assert(false,'OBSERVED|dupAssignBlock|'+ResidualRun.run('dupAssignBlock')); }
 @IsTest static void dupAssignBlockAllowSave(){ System.assert(false,'OBSERVED|dupAssignBlockAllowSave|'+ResidualRun.run('dupAssignBlockAllowSave')); }
 @IsTest static void dupAssignAlert(){ System.assert(false,'OBSERVED|dupAssignAlert|'+ResidualRun.run('dupAssignAlert')); }
 @IsTest static void dupAssignAlertAllowSave(){ System.assert(false,'OBSERVED|dupAssignAlertAllowSave|'+ResidualRun.run('dupAssignAlertAllowSave')); }
 @IsTest static void cdcAssign(){ Test.enableChangeDataCapture(); Test.getEventBus().deliver(); ResidualLog.rows.clear(); Lead l=new Lead(LastName='B',Company='RouteCdc'); Database.insert(l,opts(true,false)); Test.getEventBus().deliver(); obs('cdcAssign',new Map<String,Object>{'events'=>ResidualLog.rows,'queueOwner'=>[SELECT OwnerId FROM Lead WHERE Id=:l.Id].OwnerId==queueId()}); }
 @IsTest static void cdcDelUndel(){ Test.enableChangeDataCapture(); Parcel__c p=new Parcel__c(Name='a',Tag__c='x'); Database.insert(p,AccessLevel.SYSTEM_MODE); Test.getEventBus().deliver(); ResidualLog.rows.clear(); Database.delete(p,AccessLevel.SYSTEM_MODE); Test.getEventBus().deliver(); List<String> d=ResidualLog.rows.clone(); ResidualLog.rows.clear(); Database.undelete(p,AccessLevel.SYSTEM_MODE); Test.getEventBus().deliver(); obs('cdcDelUndel',new Map<String,Object>{'delete'=>d,'undelete'=>ResidualLog.rows}); }
 @IsTest static void cdcFlowCreate(){ Test.enableChangeDataCapture(); Test.getEventBus().deliver(); ResidualLog.rows.clear(); insert new Lead(LastName='B',Company='FlowAfter'); Test.getEventBus().deliver(); obs('cdcFlowCreate',new Map<String,Object>{'events'=>ResidualLog.rows}); }
 @IsTest static void cdcDupBlocked(){ seedDup(); Test.enableChangeDataCapture(); Test.getEventBus().deliver(); ResidualLog.rows.clear(); Database.insert(new Lead(LastName='B',Company='Blocked',Email='dup@probe.test'),opts(false,false)); Test.getEventBus().deliver(); obs('cdcDupBlocked',new Map<String,Object>{'events'=>ResidualLog.rows}); }
 @IsTest static void cdcDupAllowSave(){ seedDup(); Test.enableChangeDataCapture(); Test.getEventBus().deliver(); ResidualLog.rows.clear(); Database.insert(new Lead(LastName='B',Company='Saved',Phone='5551'),opts(false,true)); Test.getEventBus().deliver(); obs('cdcDupAllowSave',new Map<String,Object>{'events'=>ResidualLog.rows}); }
 @IsTest static void cdcMerge(){ Lead a=new Lead(LastName='M',Company='MA'); Lead b=new Lead(LastName='M',Company='MB'); insert new List<Lead>{a,b}; Test.enableChangeDataCapture(); Test.getEventBus().deliver(); ResidualLog.rows.clear(); merge a b; Test.getEventBus().deliver(); obs('cdcMerge',new Map<String,Object>{'events'=>ResidualLog.rows,'left'=>[SELECT count() FROM Lead]}); }
 @IsTest static void dupFlowAlert(){ System.assert(false,'OBSERVED|dupFlowAlert|'+ResidualRun.run('dupFlowAlert')); }
 @IsTest static void dupFlowAlertAllowSave(){ System.assert(false,'OBSERVED|dupFlowAlertAllowSave|'+ResidualRun.run('dupFlowAlertAllowSave')); }
 @IsTest static void dupFlowUpdate(){ System.assert(false,'OBSERVED|dupFlowUpdate|'+ResidualRun.run('dupFlowUpdate')); }
 @IsTest static void dupFlowCreateAllowSave(){ System.assert(false,'OBSERVED|dupFlowCreateAllowSave|'+ResidualRun.run('dupFlowCreateAllowSave')); }
 @IsTest static void dupApexUpdate(){ System.assert(false,'OBSERVED|dupApexUpdate|'+ResidualRun.run('dupApexUpdate')); }
 @IsTest static void flowApexThrow(){ System.assert(false,'OBSERVED|flowApexThrow|'+ResidualRun.run('flowApexThrow')); }
 @IsTest static void flowApexThrowSave(){ System.assert(false,'OBSERVED|flowApexThrowSave|'+ResidualRun.run('flowApexThrowSave')); }
 @IsTest static void flowApexThrowCaught(){ System.assert(false,'OBSERVED|flowApexThrowCaught|'+ResidualRun.run('flowApexThrowCaught')); }
 @IsTest static void criteria(){ System.assert(false,'OBSERVED|criteria|'+ResidualRun.run('criteria')); }
}
