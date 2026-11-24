public class CritCase {
 static String res(Database.SaveResult r) { return r.isSuccess() ? 'ok' : 'ERR:' + String.valueOf(r.getErrors()[0].getStatusCode()) + ':' + r.getErrors()[0].getMessage(); }
 public static String rv() {
  Account acme = new Account(Name='Acme RV'); insert acme;
  insert new Account(Name='Decoy RV');
  Contact bossA = new Contact(LastName='Boss', AccountId=acme.Id); Contact bossB = new Contact(LastName='Boss'); insert new List<Contact>{bossA, bossB};
  List<String> out = new List<String>();
  for (Contact c : new List<Contact>{new Contact(LastName='CritRV', ReportsToId=bossA.Id), new Contact(LastName='CritRV', ReportsToId=bossB.Id), new Contact(LastName='CritRV')}) {
   Database.SaveResult r = Database.insert(c, false);
   if (!r.isSuccess()) { out.add(res(r)); continue; }
   Contact got = [SELECT Description, Department FROM Contact WHERE Id = :c.Id];
   out.add(got.Description + ',' + got.Department);
  }
  return String.join(out, '|');
 }
}
