public class ApprovalIdentifierCapture {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  ProcessDefinition d=[SELECT Id FROM ProcessDefinition WHERE DeveloperName='Owned_Active' LIMIT 1];
  for(String mode:new List<String>{'developer','label','missing','id','id15','id_skip','id15_skip'}) {
   Account a=new Account(Name='Probe '+mode);insert a;Approval.ProcessSubmitRequest r=new Approval.ProcessSubmitRequest();r.setObjectId(a.Id);
   r.setProcessDefinitionNameOrId(mode=='developer'?'Owned_Active':mode=='label'?'Owned Active':(mode=='id'||mode=='id_skip')?String.valueOf(d.Id):(mode=='id15'||mode=='id15_skip')?String.valueOf(d.Id).substring(0,15):'No_Such_Process');
   if(mode=='id_skip'||mode=='id15_skip') r.setSkipEntryCriteria(true);
   Approval.ProcessResult p=Approval.process(r,false);out.put(mode,String.valueOf(p.isSuccess())+':'+(p.isSuccess()?p.getInstanceStatus():String.valueOf(p.getErrors()[0].getStatusCode())));
  }
  return out;
 }
}
