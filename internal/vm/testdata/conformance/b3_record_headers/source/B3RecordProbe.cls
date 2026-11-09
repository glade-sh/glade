public class B3RecordProbe {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Contact c=new Contact(LastName='Synthetic'+u,Email='s'+u+'@example.invalid');insert c;List<String> out=new List<String>();Integer n=0;
  for(String obj:new List<String>{'Account','Lead','Case'}) for(String flag:new List<String>{'user','other','auto'}) for(Boolean enabled:new List<Boolean>{false,true}) for(String route:new List<String>{'keyword','database'}) {
   n++;SObject row;if(obj=='Account')row=new Account(Name='Synthetic'+n);else if(obj=='Lead')row=new Lead(LastName='Synthetic'+u+n,Company='Synthetic');else row=new Case(Subject='Synthetic'+n,Status='New',Origin='Phone',ContactId=c.Id);
   Database.DMLOptions o=new Database.DMLOptions();if(flag=='user')o.EmailHeader.triggerUserEmail=enabled;if(flag=='other')o.EmailHeader.triggerOtherEmail=enabled;if(flag=='auto')o.EmailHeader.triggerAutoResponseEmail=enabled;row.setOptions(o);
   try {Boolean success;if(route=='keyword'){insert row;success=true;}else{Database.SaveResult result=Database.insert(row,false);success=result.isSuccess();}out.add(obj+'_'+flag+'_'+enabled+'_'+route+'='+success);}catch(Exception e){out.add(obj+'_'+flag+'_'+enabled+'_'+route+'=exception:'+e.getTypeName());}
  }
  return String.join(out,'|');
 }
}
