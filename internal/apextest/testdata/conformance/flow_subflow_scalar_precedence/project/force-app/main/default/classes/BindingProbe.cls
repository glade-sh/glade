public class BindingProbe {
 public static String invoke(String route, String mode, Id seedId) {
  Map<String,Object> inputs=new Map<String,Object>{'seedId'=>String.valueOf(seedId),'clearValue'=>null};
  String flowName='Binding'+mode+'Parent';
  if(route=='invocable') {
   Invocable.Action action=Invocable.Action.createCustomAction('flow',flowName);
   action.setInvocationParameter('seedId',String.valueOf(seedId));
   action.setInvocationParameter('clearValue',null);
   List<Invocable.Action.Result> results=action.invoke();
   return results[0].isSuccess() ? (String)results[0].getOutputParameters().get('result') : 'failed';
  }
  Flow.Interview interview;
  if(route=='static') {
   if(mode=='Record') interview=new Flow.Interview.BindingRecordParent(inputs);
   else interview=new Flow.Interview.BindingLookupParent(inputs);
  } else interview=Flow.Interview.createInterview(flowName,inputs);
  interview.start();
  return (String)interview.getVariableValue('result');
 }
 public static Map<String,String> observe() {
  Account seed=new Account(Name='Binding seed');insert seed;
  Map<String,String> output=new Map<String,String>();
  for(String mode:new List<String>{'Record','Lookup'}) {
   for(String route:new List<String>{'dynamic','static','invocable'}) {
    output.put('direct/'+route+'/'+mode,invoke(route,mode,seed.Id));
    for(String operation:new List<String>{'insert','update'}) {
     Contact item=new Contact(LastName=operation=='insert' ? 'Binding '+route+' '+mode : 'Unrelated',AccountId=seed.Id);
     insert item;
     if(operation=='update') {item.LastName='Binding '+route+' '+mode;update item;}
     output.put('trigger/'+route+'/'+mode+'/'+operation,[SELECT Description FROM Contact WHERE Id=:item.Id].Description);
    }
   }
  }
  return output;
 }
}
