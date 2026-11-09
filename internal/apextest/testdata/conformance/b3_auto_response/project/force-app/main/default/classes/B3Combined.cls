public class B3Combined {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());List<String> out=new List<String>();
  Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerUserEmail=true;o.EmailHeader.triggerOtherEmail=true;o.EmailHeader.triggerAutoResponseEmail=true;o.AssignmentRuleHeader.useDefaultRule=true;
  Lead l=new Lead(LastName='Synthetic'+u,Company='Synthetic',Email='s'+u+'@example.invalid');Database.SaveResult a=Database.insert(l,o);out.add('lead_combined='+a.isSuccess()+','+[SELECT COUNT() FROM Task WHERE WhoId=:l.Id]+(a.isSuccess()?'':','+a.getErrors()[0].getStatusCode()));
  Contact c=new Contact(LastName='Contact'+u,Email='c'+u+'@example.invalid');insert c;
  Case row=new Case(Subject='Synthetic',Status='New',Origin='Web',ContactId=c.Id,SuppliedEmail='r'+u+'@example.invalid');Database.SaveResult b=Database.insert(row,o);out.add('case_combined='+b.isSuccess()+','+[SELECT COUNT() FROM Task WHERE WhatId=:row.Id]+(b.isSuccess()?'':','+b.getErrors()[0].getStatusCode()));
  Database.DMLOptions aopt=new Database.DMLOptions();aopt.EmailHeader.triggerAutoResponseEmail=true;
  Case attached=new Case(Subject='Synthetic',Status='New',Origin='Web',SuppliedEmail='a'+u+'@example.invalid');attached.setOptions(aopt);try {insert attached;out.add('case_record=true,'+[SELECT COUNT() FROM Task WHERE WhatId=:attached.Id]);}catch(Exception e){out.add('case_record=exception:'+e.getTypeName());}
  return String.join(out,'|');
 }
}
