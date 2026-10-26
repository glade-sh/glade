public class BatchSummaryProbe {
 public static Map<String,Object> observe() {
  Account west=new Account(Name='West'),east=new Account(Name='East');
  insert new List<Account>{west,east};
  insert new List<Opportunity>{
   new Opportunity(Name='A',AccountId=west.Id,Amount=2,StageName='Prospecting',CloseDate=Date.today()),
   new Opportunity(Name='B',AccountId=west.Id,Amount=6,StageName='Prospecting',CloseDate=Date.today()),
   new Opportunity(Name='C',AccountId=east.Id,Amount=8,StageName='Prospecting',CloseDate=Date.today()),
   new Opportunity(Name='D',AccountId=east.Id,Amount=4,StageName='Prospecting',CloseDate=Date.today())};
  Integer before=Limits.getQueries();
  List<Opportunity> cached=[SELECT Id,AccountId,Amount FROM Opportunity ORDER BY AccountId,Id];
  SummaryCursor cursor=new SummaryCursor(cached);
  Map<Id,List<Opportunity>> grouped=cursor.groups();
  List<Account> changes=new List<Account>();
  for(Id parentId:grouped.keySet()) {
   SummaryValue value=new SummaryValue(grouped.get(parentId));
   changes.add(new Account(Id=parentId,AnnualRevenue=value.average(),NumberOfEmployees=value.count()));
  }
  update changes;
  Integer calcQueries=Limits.getQueries()-before;
  Map<Id,Account> saved=new Map<Id,Account>([SELECT Id,AnnualRevenue,NumberOfEmployees FROM Account WHERE Id IN :new Set<Id>{west.Id,east.Id}]);
  return new Map<String,Object>{'westAverage'=>saved.get(west.Id).AnnualRevenue,'eastAverage'=>saved.get(east.Id).AnnualRevenue,
    'westCount'=>saved.get(west.Id).NumberOfEmployees,'eastCount'=>saved.get(east.Id).NumberOfEmployees,
    'westLast'=>new SummaryValue(grouped.get(west.Id)).last(),'eastLast'=>new SummaryValue(grouped.get(east.Id)).last(),'calcQueries'=>calcQueries};
 }
}
