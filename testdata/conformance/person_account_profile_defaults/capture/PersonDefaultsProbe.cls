public class PersonDefaultsProbe {
 public static List<String> observe(String prefix) {
  Map<Id,RecordType> types=new Map<Id,RecordType>([SELECT Id,DeveloperName FROM RecordType WHERE SobjectType='Account']);
  Id patient; Id provider;
  for(RecordType rt:types.values()){if(rt.DeveloperName=='Patient'){patient=rt.Id;}if(rt.DeveloperName=='Provider'){provider=rt.Id;}}
  List<String> names=new List<String>{'person_omitted','business_omitted','person_explicit','person_missing_last','empty','person_null'};
  List<Account> accounts=new List<Account>{new Account(LastName='Probe'),new Account(Name='Probe'),new Account(LastName='Explicit',RecordTypeId=patient),new Account(FirstName='Missing',RecordTypeId=provider),new Account(),new Account(LastName='Null',RecordTypeId=null)};
  List<String> rows=new List<String>();
  for(Integer i=0;i<accounts.size();i++){
   Database.SaveResult saved=Database.insert(accounts[i],false);
   String value;
   if(saved.isSuccess()){
    Id accountId=accounts[i].Id;
    Account a=[SELECT RecordTypeId,IsPersonAccount FROM Account WHERE Id=:accountId];
    value=types.get(a.RecordTypeId).DeveloperName+'|'+a.IsPersonAccount;
   }else{
    value=String.valueOf(saved.getErrors()[0].getStatusCode())+'|'+String.join(saved.getErrors()[0].getFields(),',')+'|message='+saved.getErrors()[0].getMessage();
   }
   rows.add(prefix+'_'+names[i]+'|'+value);
  }
  return rows;
 }
}
