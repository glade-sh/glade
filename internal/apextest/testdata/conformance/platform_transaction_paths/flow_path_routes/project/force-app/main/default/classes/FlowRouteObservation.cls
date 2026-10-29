public class FlowRouteObservation {
 public static Map<String,Object> observe(String key) {
  Map<String,Object> out=new Map<String,Object>();
  try {
   if(key.startsWith('S')) {
    if(key=='S02'||key=='S04'||key=='S05') Test.startTest();
    Datetime due=key=='S04'?System.now().addDays(2):(key=='S05'?null:System.now().addDays(-2));
    CaptureRecord__c row=new CaptureRecord__c(Name='scheduled',NextRun__c=due); insert row;
    row=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Id=:row.Id];
    out.put('immediateBefore',row.Immediate__c); out.put('delayedBefore',row.Marker__c);
    if(key=='S03') Test.startTest();
    if(key!='S01') Test.stopTest();
    row=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Id=:row.Id];
    out.put('immediateAfter',row.Immediate__c); out.put('delayedAfter',row.Marker__c);
   } else {
    Flow.Interview interview;
    if(key=='I01') interview=Flow.Interview.createInterview('CaptureAsync',new Map<String,Object>());
    if(key=='I02') interview=Flow.Interview.createInterview('', 'CaptureAsync',new Map<String,Object>());
    if(key=='I03') interview=Flow.Interview.createInterview(null, 'CaptureAsync',new Map<String,Object>());
    if(key=='I04') interview=Flow.Interview.createInterview('sprig', 'CaptureAsync',new Map<String,Object>());
    if(key=='I05') interview=Flow.Interview.createInterview('', 'CaptureScheduled',new Map<String,Object>());
    if(key=='I06') interview=Flow.Interview.createInterview(null, 'CaptureScheduled',new Map<String,Object>());
    if(key=='I07') interview=Flow.Interview.createInterview('sprig', 'CaptureScheduled',new Map<String,Object>());
    if(key=='I08') interview=Flow.Interview.createInterview('', 'BirchAsync',new Map<String,Object>());
    if(key=='I09') interview=Flow.Interview.createInterview(null, 'BirchAsync',new Map<String,Object>());
    if(key=='I10') interview=Flow.Interview.createInterview('sprig', 'BirchAsync',new Map<String,Object>());
    if(key=='I11') interview=Flow.Interview.createInterview('sprig.CaptureAsync',new Map<String,Object>());
    out.put('created',interview!=null); interview.start();
   }
  } catch(Exception failure) {out.put('exception',failure.getTypeName());}
  return out;
 }
 public static void emit(String key) {System.assert(false,'FLOW_ROUTE|'+key+'|'+JSON.serialize(observe(key)));}
}
