public class ApprovalSkipCapture {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  for (String mode:new List<String>{'named_true','named_false','unnamed_true'}) {
   Contact c=new Contact(LastName='Probe '+mode); insert c;
   Approval.ProcessSubmitRequest r=new Approval.ProcessSubmitRequest(); r.setObjectId(c.Id);
   if (mode!='unnamed_true') r.setProcessDefinitionNameOrId('Criteria_False');
   r.setSkipEntryCriteria(mode!='named_false');
   try { Approval.ProcessResult p=Approval.process(r,false); out.put(mode,String.valueOf(p.isSuccess())+':'+(p.isSuccess()?p.getInstanceStatus():String.valueOf(p.getErrors()[0].getStatusCode()))); }
   catch(Exception e) { out.put(mode,e.getTypeName()+':'+e.getMessage()); }
  }
  return out;
 }
}
