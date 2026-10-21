public class AggregateCapabilitiesProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> output=new Map<String,Object>();
  Map<String,Schema.SObjectType> objects=Schema.getGlobalDescribe();
  for(String pair:new List<String>{'Task.ActivityDate','ContactPointAddress.BestTimeToContactEndTime','Task.CreatedDate','Account.Name'}) {
   List<String> parts=pair.split('\\.');
   Schema.DescribeFieldResult field=objects.get(parts[0]).getDescribe().fields.getMap().get(parts[1]).getDescribe();
   output.put(pair+'.aggregatable',field.isAggregatable());
   for(String operation:new List<String>{'MIN','MAX','COUNT','COUNT_DISTINCT'}) {
    try { Database.query('SELECT '+operation+'('+parts[1]+') value FROM '+parts[0]); output.put(pair+'.'+operation,'accepted'); }
    catch(QueryException ex) { output.put(pair+'.'+operation,ex.getMessage()); }
   }
  }
  return output;
 }
}
