public class RelationshipFilterCase {
 public static String run() {
  List<Account> parents = new List<Account>{new Account(Name='One'),new Account(Name='Other')}; insert parents;
  insert new Contact(LastName='SourceProbe', AccountId=parents[0].Id);
  List<String> output = new List<String>();
  for (Integer i=0; i<3; i++) {
   Contact c = new Contact(LastName='FilterCheck'); if (i<2) c.AccountId=parents[i].Id;
   Database.SaveResult r = Database.insert(c,false);
   if (r.isSuccess()) output.add([SELECT Description FROM Contact WHERE Id=:c.Id].Description);
   else output.add('error:' + String.valueOf(r.getErrors()[0].getStatusCode()));
  }
  return String.join(output,'|') + '|remaining=' + String.valueOf([SELECT count() FROM Contact WHERE LastName = 'FilterCheck']);
 }
}
