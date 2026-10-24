public class FlowIntegerErrorProbe {
 public static Object observe(String route, String input) {
  FlowIntegerProbe.seen=null;
  FlowValueRecord__c record=new FlowValueRecord__c(Name='integer-boundary',Whole__c=7,Fraction__c=7.75);
  String family='IntegerFraction';
  if(input=='overflow') { family='IntegerWhole'; record.Whole__c=2147483648L; }
  if(input=='maximum') { family='IntegerWhole'; record.Whole__c=2147483647; }
  if(input=='minimum') { family='IntegerWhole'; record.Whole__c=-2147483648L; }
  if(input=='underflow') { family='IntegerWhole'; record.Whole__c=-2147483649L; }
  if(input=='null') record.Fraction__c=null;
  if(input=='integral') record.Fraction__c=7.00;
  if(input=='negative') record.Fraction__c=-7.75;
  Boolean triggerRoute=route!='interview' && route!='action';
  Boolean updateRoute=route.contains('Update') || route.startsWith('update');
  Boolean databaseRoute=route.startsWith('database');
  Boolean listRoute=route.contains('List');
  Boolean upsertRoute=route.contains('Upsert') || route.startsWith('upsert');
  if(updateRoute) insert record;
  if(triggerRoute) record.Mode__c=family+(route.endsWith('Subflow') || route=='subflow' ? 'subflow' : 'trigger');
  Map<String,Object> observation=new Map<String,Object>{'exceptionType'=>null,'status'=>null,'actionSuccess'=>null,'actionErrorCode'=>null,'actionErrorCount'=>null,'interviewStatus'=>null,'saveSuccess'=>null,'saveSuccessList'=>null,'statusList'=>null};
  try {
   if(databaseRoute && listRoute) {
    FlowValueRecord__c control=new FlowValueRecord__c(Name='integer-control',Whole__c=record.Whole__c,Fraction__c=7.00,Mode__c=record.Mode__c);
    List<Database.SaveResult> saved=Database.insert(new List<FlowValueRecord__c>{record,control},false);
    List<Boolean> successes=new List<Boolean>(); List<String> codes=new List<String>();
    for(Database.SaveResult item:saved) { successes.add(item.isSuccess());codes.add(item.isSuccess() ? null : String.valueOf(item.getErrors()[0].getStatusCode())); }
    observation.put('saveSuccessList',successes);observation.put('statusList',codes);
   } else if(databaseRoute && upsertRoute) {
    Database.UpsertResult saved=Database.upsert(record,false);
    observation.put('saveSuccess',saved.isSuccess());
    if(!saved.isSuccess()) observation.put('status',String.valueOf(saved.getErrors()[0].getStatusCode()));
   } else if(databaseRoute) {
    Database.SaveResult saved=updateRoute ? Database.update(record,false) : Database.insert(record,false);
    observation.put('saveSuccess',saved.isSuccess());
    if(!saved.isSuccess()) observation.put('status',String.valueOf(saved.getErrors()[0].getStatusCode()));
   } else {
    if(upsertRoute) { upsert record; } else if(updateRoute) { update record; } else { insert record; }
    if(route=='interview') {
     Flow.Interview interview=Flow.Interview.createInterview('Value'+family,new Map<String,Object>{'InputRecord'=>record}); interview.start();
    }
    if(route=='action') {
     Invocable.Action action=Invocable.Action.createCustomAction('flow','Value'+family);
     action.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'InputRecord'=>record}});
     Invocable.Action.Result outcome=action.invoke()[0];
     observation.put('actionSuccess',outcome.isSuccess());
     observation.put('actionErrorCount',outcome.getErrors()==null ? 0 : outcome.getErrors().size());
     if(outcome.getErrors()!=null && !outcome.getErrors().isEmpty()) observation.put('actionErrorCode',outcome.getErrors()[0].getCode());
     observation.put('interviewStatus',outcome.getOutputParameters().get('Flow__InterviewStatus'));
    }
   }
  } catch(Exception failure) {
   observation.put('exceptionType',failure.getTypeName());
   if(failure instanceof DmlException) { DmlException typed=(DmlException)failure; observation.put('status',String.valueOf(typed.getDmlStatusCode(0))); }
  }
  observation.put('seen',FlowIntegerProbe.seen);
  observation.put('storedRows',[SELECT count() FROM FlowValueRecord__c WHERE Name='integer-boundary']);
  return observation;
 }
}
