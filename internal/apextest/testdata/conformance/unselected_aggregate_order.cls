@IsTest
private class OwnedAggregateOrderProbe {
 @IsTest static void queryOrder() {
  List<Account> seeds = new List<Account>{
   new Account(Name='Alpha', AnnualRevenue=10),
   new Account(Name='Alpha', AnnualRevenue=40),
   new Account(Name='Beta', AnnualRevenue=20),
   new Account(Name='Gamma')};
  insert seeds;
  Test.setCreatedDate(seeds[0].Id, Datetime.newInstanceGmt(2024,1,1,0,0,0));
  Test.setCreatedDate(seeds[1].Id, Datetime.newInstanceGmt(2024,1,4,0,0,0));
  Test.setCreatedDate(seeds[2].Id, Datetime.newInstanceGmt(2024,1,2,0,0,0));
  Test.setCreatedDate(seeds[3].Id, Datetime.newInstanceGmt(2024,1,3,0,0,0));
  List<String> queries = new List<String>{
   'SELECT Name FROM Account GROUP BY Name ORDER BY MIN(CreatedDate) DESC LIMIT 2',
   'SELECT Name FROM Account GROUP BY Name ORDER BY MIN(AnnualRevenue) DESC NULLS LAST',
   'SELECT Name FROM Account GROUP BY Name ORDER BY MAX(AnnualRevenue) DESC NULLS LAST',
   'SELECT Name FROM Account GROUP BY Name ORDER BY SUM(AnnualRevenue) DESC NULLS LAST',
   'SELECT Name FROM Account GROUP BY Name ORDER BY AVG(AnnualRevenue) DESC NULLS LAST',
   'SELECT Name FROM Account GROUP BY Name ORDER BY COUNT(Id) DESC, Name',
   'SELECT Name FROM Account GROUP BY Name HAVING MIN(AnnualRevenue) > 0 ORDER BY MIN(AnnualRevenue) DESC',
   'SELECT Name, COUNT(Id) total FROM Account GROUP BY Name ORDER BY MIN(CreatedDate) DESC',
   'SELECT Name, MIN(AnnualRevenue) lowest FROM Account GROUP BY Name ORDER BY MIN(AnnualRevenue) DESC NULLS LAST',
   'SELECT Name FROM Account GROUP BY Name ORDER BY MIN(AnnualRevenue) ASC NULLS FIRST'};
  List<String> expected = new List<String>{'Gamma,Beta','Beta,Alpha,Gamma','Alpha,Beta,Gamma','Alpha,Beta,Gamma','Alpha,Beta,Gamma','Alpha,Beta,Gamma','Beta,Alpha','Gamma,Beta,Alpha','Beta,Alpha,Gamma','Gamma,Alpha,Beta'};
  for(Integer i=0;i<queries.size();i++) {
   List<AggregateResult> rows = Database.query(queries[i]);
   List<String> names = new List<String>();
   for(AggregateResult row:rows) {
    names.add((String)row.get('Name'));
    if(!queries[i].contains(' total') && !queries[i].contains(' lowest')) {
     System.assertEquals(new Set<String>{'Name'}, row.getPopulatedFieldsAsMap().keySet());
    }
   }
   System.assertEquals(expected[i],String.join(names,','),'case '+i);
  }
  List<AggregateResult> literalRows = [SELECT Name FROM Account GROUP BY Name ORDER BY MIN(CreatedDate) DESC LIMIT 2];
  System.assertEquals('Gamma', literalRows[0].get('Name'));
  System.assertEquals('Beta', literalRows[1].get('Name'));
 }
}
