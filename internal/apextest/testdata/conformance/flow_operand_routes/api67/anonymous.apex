Map<String,Object> rows=new Map<String,Object>();
for(String route:new List<String>{'factory','typed','action','before','after','subflow'}) {
 try {
  if(route=='factory') { Flow.Interview f=Flow.Interview.createInterview('Cedar_Operands',new Map<String,Object>());f.start();rows.put(route,f.getVariableValue('Result')); }
  else if(route=='typed') { Flow.Interview.Cedar_Operands f=new Flow.Interview.Cedar_Operands(new Map<String,Object>());f.start();rows.put(route,f.getVariableValue('Result')); }
  else if(route=='action') { Invocable.Action a=Invocable.Action.createCustomAction('flow','Cedar_Operands');a.setInvocations(new List<Map<String,Object>>{new Map<String,Object>()});Invocable.Action.Result r=a.invoke()[0];rows.put(route,r.isSuccess()?r.getOutputParameters().get('Result'):JSON.serialize(r.getErrors())); }
  else { Contact c=new Contact(LastName='Cedar'+(route=='before'?'Before':route=='after'?'After':'Subflow'));insert c;rows.put(route,[SELECT Description FROM Contact WHERE Id=:c.Id].Description); }
 } catch(Exception e) { rows.put(route,e.getTypeName()+':'+e.getMessage()); }
}
System.assert(false,'CEDAR_ROUTES|'+JSON.serialize(rows));
