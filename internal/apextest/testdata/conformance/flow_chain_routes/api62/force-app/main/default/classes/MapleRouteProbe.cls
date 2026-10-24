public class MapleRouteProbe {
 public static String observe() {
  Map<String,Object> rows=new Map<String,Object>();
  for (String route : new List<String>{'factory','typed','action','trigger_subflow'}) {
   for (Boolean found : new List<Boolean>{false,true}) {
    if(found) insert new Contact(LastName='Match',Title='Guide');
    Map<String,Object> cell=new Map<String,Object>();
    Map<String,Object> inputs=new Map<String,Object>{'SearchKey'=>found?'Match':'Absent'};
    try {
     if(route=='trigger_subflow') {
      Account record=new Account(Name='Maple route',Site=found?'Match':'Absent'); insert record;
      record=[SELECT Description,Site FROM Account WHERE Id=:record.Id];
      cell.put('text',record.Description);cell.put('status',record.Site);cell.put('number',null);
     } else if(route=='action') {
      Invocable.Action action=Invocable.Action.createCustomAction('flow','MapleLeaf');
      action.setInvocations(new List<Map<String,Object>>{inputs});
      List<Invocable.Action.Result> results=action.invoke();
      if(!results[0].isSuccess()) cell.put('actionFailed',true);
      Map<String,Object> output=results[0].getOutputParameters();
      cell.put('text',output.get('Result'));cell.put('status',output.get('Status'));cell.put('number',output.get('Count'));
     } else {
      Flow.Interview interview=route=='typed'?new Flow.Interview.MapleLeaf(inputs):Flow.Interview.createInterview('MapleLeaf',inputs);
      interview.start();
      cell.put('text',interview.getVariableValue('Result'));cell.put('status',interview.getVariableValue('Status'));cell.put('number',interview.getVariableValue('Count'));
     }
    } catch(Exception e) {cell.put('exception',e.getTypeName());}
    rows.put(route+'_'+String.valueOf(found),cell);
   }
  }
  return JSON.serialize(rows);
 }
}
