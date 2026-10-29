public class FlowProfileContext67 {
 public static Map<String,Object> current() {
  String profile = [SELECT Name FROM Profile WHERE Id=:UserInfo.getProfileId()].Name;
  String userId = UserInfo.getUserId();
  Map<String,Object> row = new Map<String,Object>();
  row.put('match', save(profile,userId));
  row.put('wrongProfile', save(profile=='Standard User'?'System Administrator':'Standard User',userId));
  row.put('wrongUser', save(profile,''));
  return row;
 }
 public static List<Boolean> save(String profile,String userId) {
  List<FlowProfileRecord__c> records = new List<FlowProfileRecord__c>();
  for(String name:new List<String>{'entry','decision','workflow','ordinary'}) records.add(new FlowProfileRecord__c(Name=name,ExpectedProfile__c=profile,ExpectedUser__c=userId));
  insert records;
  List<Boolean> observed = new List<Boolean>();
  for(FlowProfileRecord__c item:[SELECT Entry__c,Decision__c,Workflow__c FROM FlowProfileRecord__c WHERE Id IN :records ORDER BY Name]) {
   observed.add(item.Entry__c); observed.add(item.Decision__c); observed.add(item.Workflow__c);
  }
  return observed;
 }
}
