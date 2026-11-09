public class B3CaseRecipient {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());List<String> out=new List<String>();
  for(Integer i=0;i<2;i++) {Contact c=new Contact(LastName='Synthetic'+u+i,Email=i==0?'s'+u+'@example.invalid':null);insert c;
   Case row=new Case(Subject='Synthetic',Status='New',Origin='Web',ContactId=c.Id);Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;
   Database.SaveResult r=Database.insert(row,o);out.add('contact_'+i+'='+r.isSuccess()+','+[SELECT COUNT() FROM Task WHERE WhatId=:row.Id]+(r.isSuccess()?'':','+r.getErrors()[0].getStatusCode()));
  }return String.join(out,'|');
 }
}
