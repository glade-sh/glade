public class SummaryCursor {
 private List<Opportunity> cached;
 public SummaryCursor(List<Opportunity> rows) {cached=rows;}
 public Map<Id,List<Opportunity>> groups() {
  Map<Id,List<Opportunity>> result=new Map<Id,List<Opportunity>>();
  for(Opportunity row:cached) {
   if(!result.containsKey(row.AccountId)) result.put(row.AccountId,new List<Opportunity>());
   result.get(row.AccountId).add(row);
  }
  return result;
 }
}
