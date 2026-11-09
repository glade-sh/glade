public class B3Probe {
 public static String run() {
  List<String> out=new List<String>();
  for(String obj:new List<String>{'Account','Lead','Case'}) {
   for(String flag:new List<String>{'user','other','auto'}) {
    for(Boolean enabled:new List<Boolean>{false,true}) {
     SObject row; if(obj=='Account') row=new Account(Name='Synthetic'); else if(obj=='Lead') row=new Lead(LastName='Synthetic',Company='Synthetic'); else row=new Case(Subject='Synthetic',Status='New',Origin='Phone');
     Database.DMLOptions o=new Database.DMLOptions();
     if(flag=='user') o.EmailHeader.triggerUserEmail=enabled;
     if(flag=='other') o.EmailHeader.triggerOtherEmail=enabled;
     if(flag=='auto') o.EmailHeader.triggerAutoResponseEmail=enabled;
     try { Database.SaveResult r=Database.insert(row,o); out.add(obj+'_'+flag+'_'+enabled+'='+r.isSuccess()+(r.isSuccess()?'':','+r.getErrors()[0].getStatusCode()+','+r.getErrors()[0].getMessage())); }
     catch(Exception e) {out.add(obj+'_'+flag+'_'+enabled+'=exception:'+e.getTypeName()+':'+e.getMessage());}
    }
   }
  }
  Database.DMLOptions a=new Database.DMLOptions(); a.AssignmentRuleHeader.useDefaultRule=true;
  try { Database.SaveResult r=Database.insert(new Account(Name='Synthetic assignment'),a); out.add('account_assignment='+r.isSuccess()); } catch(Exception e) {out.add('account_assignment=exception:'+e.getTypeName()+':'+e.getMessage());}
  Database.DMLOptions p=new Database.DMLOptions(); p.EmailHeader.triggerOtherEmail=true;
  try { Database.SaveResult[] rs=Database.insert(new List<Account>{new Account(Name='Synthetic valid'),new Account()},p); out.add('partial='+rs[0].isSuccess()+','+rs[1].isSuccess()+','+rs[1].getErrors()[0].getStatusCode()); } catch(Exception e) {out.add('partial=exception:'+e.getTypeName()+':'+e.getMessage());}
  return String.join(out,'|');
 }
}
