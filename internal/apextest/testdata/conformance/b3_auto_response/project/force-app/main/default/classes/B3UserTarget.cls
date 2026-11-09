public class B3UserTarget {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Id q=[SELECT Id FROM Group WHERE DeveloperName='SyntheticRouting' AND Type='Queue' LIMIT 1].Id;List<String> out=new List<String>();
  for(String obj:new List<String>{'Lead','Case'}) for(String mode:new List<String>{'default','active','inactive','false'}) {
   SObject row;if(obj=='Lead')row=new Lead(LastName='Synthetic'+u+mode,Company='Synthetic',OwnerId=q);else row=new Case(Subject='Synthetic',Status='New',Origin='Phone',OwnerId=q);
   Database.DMLOptions o=new Database.DMLOptions();if(mode=='default')o.AssignmentRuleHeader.useDefaultRule=true;else if(mode=='false')o.AssignmentRuleHeader.useDefaultRule=false;else {String name=mode=='active'?'B3ActiveUser':'B3InactiveUser';o.AssignmentRuleHeader.assignmentRuleId=[SELECT Id FROM AssignmentRule WHERE SobjectType=:obj AND Name=:name LIMIT 1].Id;}
   Database.SaveResult r=Database.insert(row,o);SObject stored=Database.query('SELECT OwnerId FROM '+obj+' WHERE Id=\''+row.Id+'\'');out.add(obj+'_'+mode+'='+r.isSuccess()+','+((Id)stored.get('OwnerId')==UserInfo.getUserId())+','+((Id)stored.get('OwnerId')==q));
  }return String.join(out,'|');
 }
}
