public class CapturePlatformProbe {
 public static Integer cdcCount = 0;
 public class Reply implements HttpCalloutMock {
  public HttpResponse respond(HttpRequest request) { HttpResponse r=new HttpResponse(); r.setStatusCode(204); return r; }
 }
 public static Map<String,Object> observe(String key) {
  Map<String,Object> out=new Map<String,Object>{'exception'=>null};
  try {
   if(key.startsWith('C')) {
    cdcCount=0;
    if(key!='C01') Test.enableChangeDataCapture();
    if(key=='C03') Test.startTest();
    if(key!='C04') insert new Account(Name='synthetic change');
    out.put('before', [SELECT count() FROM CaptureLog__c WHERE Name='cdc']);
    if(key=='C03') Test.stopTest(); else Test.getEventBus().deliver();
    out.put('after', [SELECT count() FROM CaptureLog__c WHERE Name='cdc']); out.put('static',cdcCount);
   } else if(key.startsWith('F')) {
    if(key=='F06') {
     Flow.Interview.createInterview('CaptureAsync',new Map<String,Object>()).start();
    } else {
     if(key=='F02'||key=='F04') Test.startTest();
     String mode=(key=='F01'||key=='F02')?'async':((key=='F05')?'timer':'scheduled');
     CaptureRecord__c row=new CaptureRecord__c(Name=mode,NextRun__c=System.now().addDays(2)); insert row;
     if(key=='F02'||key=='F04') Test.stopTest();
     row=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Id=:row.Id];
     out.put('immediate',row.Immediate__c); out.put('delayed',row.Marker__c);
     out.put('timerLogs',[SELECT count() FROM CaptureLog__c WHERE Name='timer']);
    }
   } else if(key.startsWith('P')) {
    String runKey=key+'-'+String.valueOf(Crypto.getRandomInteger());
    if(key=='P07') Test.startTest();
    if(key!='P06') {
     List<CapturePulse__e> events=new List<CapturePulse__e>{new CapturePulse__e(Key__c=key=='P05'?'reject':'accept',RunKey__c=runKey)};
     if(key=='P04') events.add(new CapturePulse__e(Key__c='accept',RunKey__c=runKey));
     List<Database.SaveResult> results=EventBus.publish(events); out.put('published',results[0].isSuccess());
    }
    out.put('before',[SELECT count() FROM CaptureLog__c WHERE Name=:runKey]);
    if(key=='P07') Test.stopTest(); else if(key!='P01') Test.getEventBus().deliver();
    if(key=='P03') Test.getEventBus().deliver();
    out.put('after',[SELECT count() FROM CaptureLog__c WHERE Name=:runKey]);
   } else if(key.startsWith('D')) {
    String token=key=='D04'?null:'same-key';
    insert new CaptureMatch__c(Name='first',MatchKey__c=token);
    CaptureMatch__c row=new CaptureMatch__c(Name='second',MatchKey__c=(key=='D01'||key=='D05')?'other-key':token);
    Database.DMLOptions options=new Database.DMLOptions(); options.DuplicateRuleHeader.allowSave=(key=='D03');
    Database.SaveResult result;
    if(key=='D05') {insert row; row.MatchKey__c=token; result=Database.update(row,false);} else result=Database.insert(row,options);
    out.put('success',result.isSuccess()); if(!result.isSuccess()) out.put('status',String.valueOf(result.getErrors()[0].getStatusCode()));
    out.put('count',[SELECT count() FROM CaptureMatch__c]);
   } else if(key.startsWith('A')) {
    Lead row=new Lead(LastName='Synthetic',Company=key=='A03'?'route-miss':'route-match');
    Database.DMLOptions options=new Database.DMLOptions();
    if(key=='A02'||key=='A03') options.assignmentRuleHeader.useDefaultRule=true;
    if(key=='A04') options.assignmentRuleHeader.assignmentRuleId=[SELECT Id FROM AssignmentRule WHERE SobjectType='Lead' AND Name='CaptureLeadRouting' LIMIT 1].Id;
    if(key=='A05') options.assignmentRuleHeader.assignmentRuleId='01Q000000000001';
    Database.SaveResult result=Database.insert(row,options); out.put('success',result.isSuccess());
    if(result.isSuccess()) {row=[SELECT OwnerId FROM Lead WHERE Id=:result.Id]; out.put('queue',String.valueOf(row.OwnerId).startsWith('00G'));}
    else out.put('status',String.valueOf(result.getErrors()[0].getStatusCode()));
   } else if(key.startsWith('W')) {
    if(key=='W03'||key=='W04'||key=='W05'||key=='W06') Test.setMock(HttpCalloutMock.class,new Reply());
    Savepoint point;
    if(key=='W05'||key=='W06') point=Database.setSavepoint();
    if(key!='W01'&&key!='W03') insert new Account(Name='pending work');
    if(point!=null) Database.rollback(point);
    if(key=='W06') Database.releaseSavepoint(point);
    HttpRequest request=new HttpRequest(); request.setEndpoint('https://native.invalid'); request.setMethod('GET');
    try {out.put('status',new Http().send(request).getStatusCode());}
    catch(Exception failure) {out.put('exception',failure.getTypeName());out.put('pending',failure.getMessage().contains('uncommitted work'));out.put('testUnsupported',failure.getMessage().contains('TestMethod'));out.put('endpointRejected',failure.getMessage().contains('Unauthorized endpoint'));}
   }
  } catch(Exception failure) {out.put('exception',failure.getTypeName());}
  return out;
 }
 public static void emit(String key) {System.assert(false,'PLATFORM_OBS|'+key+'|'+JSON.serialize(observe(key)));}
}
