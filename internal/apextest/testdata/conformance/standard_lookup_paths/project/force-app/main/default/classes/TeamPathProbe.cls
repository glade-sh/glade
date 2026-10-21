public class TeamPathProbe {
 public static Map<String,Object> observe() {
  List<AccountTeamMember> rows = [SELECT Id, Account.ParentId, Account.Name, User.Id FROM AccountTeamMember WHERE Account.Name = 'Owned path probe' LIMIT 1];
  Map<String,Object> result = new Map<String,Object>();
  result.put('empty', rows.isEmpty());
  result.put('accountRelation', AccountTeamMember.AccountId.getDescribe().getRelationshipName());
  result.put('userRelation', AccountTeamMember.UserId.getDescribe().getRelationshipName());
  return result;
 }
}
