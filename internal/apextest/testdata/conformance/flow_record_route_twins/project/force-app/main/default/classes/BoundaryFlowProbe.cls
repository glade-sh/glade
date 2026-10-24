public class BoundaryFlowProbe {
 public static Map<String,Object> observe(String route) {
  Account seed = new Account(Name='Boundary seed',Phone='111',BillingCity='Origin');
  insert seed;
  Map<String,Object> result = new Map<String,Object>();
  if (route.startsWith('interview_') || route.startsWith('invocable_')) {
   Boolean nested = route.endsWith('subflow');
   String flowName = nested ? 'BoundaryParent' : 'BoundaryLeaf';
   String outputName = nested ? 'returned' : 'candidate';
   Map<String,Object> inputs = nested ? new Map<String,Object>{'sourceName'=>seed.Name} : new Map<String,Object>{'candidate'=>seed};
   Account returned;
   if (route.startsWith('interview_')) {
    Flow.Interview interview = Flow.Interview.createInterview(flowName,inputs);
    interview.start();
    returned = (Account)interview.getVariableValue(outputName);
   } else {
    Invocable.Action action = Invocable.Action.createCustomAction('flow',flowName);
    action.setInvocationParameter(nested ? 'sourceName' : 'candidate',nested ? (Object)seed.Name : (Object)seed);
    List<Invocable.Action.Result> outcomes = action.invoke();
    result.put('success',outcomes[0].isSuccess());
    returned = (Account)outcomes[0].getOutputParameters().get(outputName);
   }
   result.put('hasRecord',returned != null);
   if (returned != null) {
    result.put('sameId',returned.Id == seed.Id);
    result.put('name',returned.Name);
    result.put('city',returned.BillingCity);
    result.put('phone',returned.Phone);
   }
   result.put('callerPhone',seed.Phone);
  } else if (route.startsWith('event_')) {
   Test.startTest();
   Database.SaveResult published = EventBus.publish(new BoundaryDirect__e(Key__c=seed.Name));
   result.put('published',published.isSuccess());
   result.put('before',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone);
   if (route.endsWith('explicit')) Test.getEventBus().deliver();
   Test.stopTest();
  } else if (route == 'scheduled_dml') {
   seed.Description='updated'; update seed;
   result.put('updated',[SELECT Description FROM Account WHERE Id=:seed.Id].Description);
  } else {
   String name = route.startsWith('before_') ? 'Boundary before' : 'Boundary '+(route.startsWith('delete_') ? 'delete ' : 'after ')+(route.contains('subflow') ? 'nested' : 'direct');
   Contact contact = new Contact(LastName=route.endsWith('update') ? 'Unrelated' : name,AccountId=seed.Id);
   insert contact;
   if (route.endsWith('update')) {contact.LastName=name; update contact;}
   if (route.startsWith('delete_')) {delete contact; result.put('deleted',[SELECT COUNT() FROM Contact WHERE Id=:contact.Id]==0);}
   else result.put('description',[SELECT Description FROM Contact WHERE Id=:contact.Id].Description);
  }
  result.put('storedPhone',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone);
  result.put('storedCity',[SELECT BillingCity FROM Account WHERE Id=:seed.Id].BillingCity);
  return result;
 }
}
