public class ApprovalCapture {
 public static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
  List<ProcessDefinition> ds=[SELECT DeveloperName, State, TableEnumOrId FROM ProcessDefinition WHERE DeveloperName IN ('Owned_Active','Owned_Inactive') ORDER BY DeveloperName];
  List<String> names=new List<String>(); for(ProcessDefinition d:ds) names.add(d.DeveloperName+':'+d.State+':'+d.TableEnumOrId); out.put('definitions',names);
  Lead lead=new Lead(LastName='Probe',Company='Probe'); insert lead;
  Approval.ProcessSubmitRequest none=new Approval.ProcessSubmitRequest(); none.setObjectId(lead.Id);
  Approval.ProcessResult nr=Approval.process(none,false); out.put('missing_no_process',String.valueOf(nr.isSuccess())+':'+String.valueOf(nr.getErrors()[0].getStatusCode()));
  Contact c=new Contact(LastName='Probe'); insert c;
  Approval.ProcessSubmitRequest missing=new Approval.ProcessSubmitRequest(); missing.setObjectId(c.Id);
  Approval.ProcessResult m=Approval.process(missing,false); out.put('missing',String.valueOf(m.isSuccess())+':'+String.valueOf(m.getErrors()[0].getStatusCode()));
  Approval.LockResult l=Approval.lock(c,false); out.put('lock',String.valueOf(l.isSuccess())+':'+String.valueOf(Approval.isLocked(c.Id)));
  Approval.LockResult l2=Approval.lock(c,false); out.put('lock_again',String.valueOf(l2.isSuccess())+':'+(l2.isSuccess()?'NONE':String.valueOf(l2.getErrors()[0].getStatusCode())));
  Approval.UnlockResult u=Approval.unlock(c,false); out.put('unlock',String.valueOf(u.isSuccess())+':'+String.valueOf(Approval.isLocked(c.Id)));
  Approval.UnlockResult u2=Approval.unlock(c,false); out.put('unlock_again',String.valueOf(u2.isSuccess())+':'+(u2.isSuccess()?'NONE':String.valueOf(u2.getErrors()[0].getStatusCode())));
  Account a=new Account(Name='Probe'); insert a;
  Approval.ProcessSubmitRequest req=new Approval.ProcessSubmitRequest(); req.setObjectId(a.Id); req.setProcessDefinitionNameOrId('Owned_Active');
  Approval.ProcessResult p=Approval.process(req,false); out.put('submit',String.valueOf(p.isSuccess())+':'+p.getInstanceStatus()+':'+String.valueOf([SELECT count() FROM ProcessInstance WHERE Id=:p.getInstanceId()]));
  return out;
 }
}
