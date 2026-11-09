public class ApprovalLockCapture {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  Lead l=new Lead(LastName='Probe',Company='Probe'); insert l;
  Approval.LockResult lr=Approval.lock(l,false); out.put('no_process_lock',String.valueOf(lr.isSuccess())+':'+String.valueOf(Approval.isLocked(l.Id)));
  Approval.UnlockResult ur=Approval.unlock(l,false); out.put('no_process_unlock',String.valueOf(ur.isSuccess())+':'+String.valueOf(Approval.isLocked(l.Id)));
  Account a=new Account(Name='Probe'); insert a;
  Approval.ProcessSubmitRequest r=new Approval.ProcessSubmitRequest(); r.setObjectId(a.Id); r.setProcessDefinitionNameOrId('Owned_Active');
  Approval.ProcessResult pr=Approval.process(r,false); out.put('pending_locked',String.valueOf(pr.isSuccess())+':'+String.valueOf(Approval.isLocked(a.Id)));
  Approval.LockResult pl=Approval.lock(a,false); out.put('pending_lock',String.valueOf(pl.isSuccess())+':'+(pl.isSuccess()?'NONE':String.valueOf(pl.getErrors()[0].getStatusCode())));
  Approval.UnlockResult pu=Approval.unlock(a,false); out.put('pending_unlock',String.valueOf(pu.isSuccess())+':'+(pu.isSuccess()?'NONE':String.valueOf(pu.getErrors()[0].getStatusCode()))+':'+String.valueOf(Approval.isLocked(a.Id)));
  for (String action:new List<String>{'Approve','Reject'}) {
   Account b=new Account(Name='Probe '+action); insert b; Approval.ProcessSubmitRequest sr=new Approval.ProcessSubmitRequest(); sr.setObjectId(b.Id); sr.setProcessDefinitionNameOrId('Owned_Active'); Approval.ProcessResult sp=Approval.process(sr,false);
   Approval.ProcessWorkitemRequest wr=new Approval.ProcessWorkitemRequest(); wr.setWorkitemId(sp.getNewWorkitemIds()[0]); wr.setAction(action); Approval.ProcessResult wp=Approval.process(wr,false);
   out.put(action.toLowerCase(),String.valueOf(wp.isSuccess())+':'+wp.getInstanceStatus()+':'+String.valueOf(Approval.isLocked(b.Id))+':'+[SELECT Status FROM ProcessInstance WHERE Id=:sp.getInstanceId()].Status);
  }
  return out;
 }
}
