public class B3FlagMatrix {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Contact c=new Contact(LastName='Synthetic'+u,Email='s'+u+'@example.invalid');insert c;List<String> out=new List<String>();
  for(String obj:new List<String>{'Lead','Case'}) for(Integer i=0;i<8;i++) for(String route:new List<String>{'options','record'}) {
   SObject row;if(obj=='Lead')row=new Lead(LastName='Synthetic'+u+i+route,Company='Synthetic');else row=new Case(Subject='Synthetic'+i+route,Status='New',Origin='Phone',ContactId=c.Id);
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerUserEmail=(i>=4);o.EmailHeader.triggerOtherEmail=(Math.mod(i,4)>=2);o.EmailHeader.triggerAutoResponseEmail=(Math.mod(i,2)==1);
   try {Boolean success;if(route=='record'){row.setOptions(o);insert row;success=true;}else{Database.SaveResult r=Database.insert(row,o);success=r.isSuccess();}out.add(obj+'_'+i+'_'+route+'='+success);}catch(Exception e){out.add(obj+'_'+i+'_'+route+'=exception:'+e.getTypeName());}
  }return String.join(out,'|');
 }
}
