public class BoundaryDeliveryProbe {
 public static Map<String,Object> eventDelivery(String route) {
  Account seed = new Account(Name='Delivery '+route,Phone='111'); insert seed;
  if (route != 'ordinary') Test.startTest();
  Database.SaveResult published = EventBus.publish(new BoundaryDirect__e(Key__c=seed.Name));
  Map<String,Object> observed = new Map<String,Object>{'published'=>published.isSuccess(),'before'=>[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone};
  if (route == 'explicit') Test.getEventBus().deliver();
  if (route != 'ordinary') Test.stopTest();
  observed.put('after',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone);
  return observed;
 }
 public static Map<String,Object> scheduledDelivery(String route) {
  if (Test.isRunningTest()) Test.startTest();
  Account seed = new Account(Name='Boundary seed',Phone='111'); insert seed;
  Map<String,Object> observed = new Map<String,Object>{'before'=>[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone};
  if (route != 'stop') {
   try {
    String flowName = route.contains('direct') ? 'BoundaryScheduledDirect' : 'BoundaryScheduledNested';
    Map<String,Object> inputs = new Map<String,Object>{'$Record'=>seed};
    Flow.Interview interview;
    if (route.endsWith('_static')) {
     if (route.contains('direct')) interview = new Flow.Interview.BoundaryScheduledDirect(inputs);
     else interview = new Flow.Interview.BoundaryScheduledNested(inputs);
    } else if (route.endsWith('_namespace')) {
     interview = Flow.Interview.createInterview('',flowName,inputs);
    } else interview = Flow.Interview.createInterview(flowName,inputs);
    interview.start();
    observed.put('started',true);
    observed.put('output',interview.getVariableValue('returned') != null);
   } catch (Exception e) { observed.put('error',e.getTypeName()); }
  }
  if (Test.isRunningTest()) Test.stopTest();
  observed.put('after',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone);
  return observed;
 }
}
