public class B3Extended {
 public static String run() {
  List<String> out=new List<String>();String u=String.valueOf(Crypto.getRandomInteger());
  Database.DMLOptions opt=new Database.DMLOptions();opt.EmailHeader.triggerAutoResponseEmail=true;
  Lead missing=new Lead(LastName='Synthetic'+u,Company='Synthetic');Database.SaveResult a=Database.insert(missing,opt);
  out.add('lead_missing='+a.isSuccess()+','+[SELECT COUNT() FROM Task WHERE WhoId=:missing.Id]+(a.isSuccess()?'':','+a.getErrors()[0].getStatusCode()));
  Case c=new Case(Subject='Synthetic',Status='New',Origin='Web');Database.SaveResult b=Database.insert(c,opt);
  out.add('case_missing='+b.isSuccess()+','+[SELECT COUNT() FROM Task WHERE WhatId=:c.Id]+(b.isSuccess()?'':','+b.getErrors()[0].getStatusCode()));
  Database.DMLOptions combined=new Database.DMLOptions();combined.EmailHeader.triggerUserEmail=true;combined.EmailHeader.triggerOtherEmail=true;combined.EmailHeader.triggerAutoResponseEmail=true;combined.AssignmentRuleHeader.useDefaultRule=true;
  try {Database.SaveResult x=Database.insert(new Account(Name='Synthetic'),combined);out.add('account_combined='+x.isSuccess());} catch(Exception e){out.add('account_combined=exception:'+e.getTypeName());}
  Lead attached=new Lead(LastName='Attached'+u,Company='Synthetic',Email='s'+u+'@example.invalid');attached.setOptions(opt);
  try {insert attached;out.add('lead_record=true,'+[SELECT COUNT() FROM Task WHERE WhoId=:attached.Id]);} catch(Exception e){out.add('lead_record=exception:'+e.getTypeName());}
  Lead good=new Lead(LastName='Partial'+u,Company='Synthetic',Email='p'+u+'@example.invalid');Lead bad=new Lead(LastName='Bad'+u);
  try {Database.SaveResult[] r=Database.insert(new List<Lead>{good,bad},opt);out.add('lead_partial='+r[0].isSuccess()+','+r[1].isSuccess()+','+r[1].getErrors()[0].getStatusCode()+','+[SELECT COUNT() FROM Task WHERE WhoId=:good.Id]);}catch(Exception e){out.add('lead_partial=exception:'+e.getTypeName());}
  return String.join(out,'|');
 }
}
