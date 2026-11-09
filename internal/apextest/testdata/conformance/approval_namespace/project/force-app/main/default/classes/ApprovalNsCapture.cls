global class ApprovalNsCapture {
 static String submit(String key) {
  Account a=new Account(Name='Probe'); insert a;
  Approval.ProcessSubmitRequest req=new Approval.ProcessSubmitRequest(); req.setObjectId(a.Id);
  if (key != null) req.setProcessDefinitionNameOrId(key);
  Approval.ProcessResult p=Approval.process(req,false);
  return String.valueOf(p.isSuccess())+':'+(p.isSuccess()?p.getInstanceStatus():String.valueOf(p.getErrors()[0].getStatusCode()));
 }
 global static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
  List<ProcessDefinition> ds=[SELECT DeveloperName, Name, State, TableEnumOrId FROM ProcessDefinition WHERE DeveloperName IN ('Owned_Active','gladens004__Owned_Active') ORDER BY DeveloperName];
  List<String> names=new List<String>(); for(ProcessDefinition d:ds) names.add(d.DeveloperName+':'+d.Name+':'+d.State+':'+d.TableEnumOrId); out.put('definitions',names);
  out.put('definition_prefixed_name_count',[SELECT count() FROM ProcessDefinition WHERE DeveloperName='gladens004__Owned_Active']);
  ProcessDefinition d=[SELECT Id FROM ProcessDefinition WHERE DeveloperName='Owned_Active' LIMIT 1];
  String id15=String.valueOf(d.Id).substring(0,15);
  out.put('submit_plain',submit('Owned_Active'));
  out.put('submit_prefixed',submit('gladens004__Owned_Active'));
  out.put('submit_lower_prefixed',submit('GLADENS004__owned_active'));
  out.put('submit_id15',submit(id15));
  out.put('submit_id18',submit(String.valueOf(d.Id)));
  out.put('submit_none',submit(null));
  return out;
 }
}
