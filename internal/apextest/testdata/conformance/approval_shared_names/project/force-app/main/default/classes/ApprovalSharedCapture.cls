public class ApprovalSharedCapture {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  List<String> defs=new List<String>();for(ProcessDefinition d:[SELECT DeveloperName,TableEnumOrId FROM ProcessDefinition WHERE DeveloperName='Shared_Review' ORDER BY TableEnumOrId]) defs.add(d.DeveloperName+':'+d.TableEnumOrId);out.put('definitions',defs);
  Account a=new Account(Name='Probe');insert a;Approval.ProcessSubmitRequest ar=new Approval.ProcessSubmitRequest();ar.setObjectId(a.Id);ar.setProcessDefinitionNameOrId('Shared_Review');Approval.ProcessResult ap=Approval.process(ar,false);out.put('account',String.valueOf(ap.isSuccess())+':'+(ap.isSuccess()?ap.getInstanceStatus():String.valueOf(ap.getErrors()[0].getStatusCode())));
  Contact c=new Contact(LastName='Probe');insert c;Approval.ProcessSubmitRequest cr=new Approval.ProcessSubmitRequest();cr.setObjectId(c.Id);cr.setProcessDefinitionNameOrId('Shared_Review');Approval.ProcessResult cp=Approval.process(cr,false);out.put('contact',String.valueOf(cp.isSuccess())+':'+(cp.isSuccess()?cp.getInstanceStatus():String.valueOf(cp.getErrors()[0].getStatusCode())));
  return out;
 }
}
