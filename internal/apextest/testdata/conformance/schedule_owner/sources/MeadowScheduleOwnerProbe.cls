public class MeadowScheduleOwnerProbe implements Schedulable {
 public void execute(SchedulableContext context) {}
 public static Map<String,Boolean> observe() {
  Map<String,Boolean> values = new Map<String,Boolean>();
  Id current = UserInfo.getUserId();
  User expected = [SELECT Name, Profile.Name FROM User WHERE Id=:current];
  values.put('SC02', expected.Name != null && expected.Profile.Name != null);
  String expression = '0 0 0 1 1 ? 2099';
  Id scheduled = System.schedule('Meadow owner schedule', expression, new MeadowScheduleOwnerProbe());
  try {
   CronTrigger row = [SELECT Id, OwnerId, CronExpression, CronJobDetail.Name FROM CronTrigger WHERE Id=:scheduled];
   values.put('SO01', row.OwnerId == current);
   List<User> owners = [SELECT Name, Profile.Name FROM User WHERE Id=:row.OwnerId];
   values.put('SO02', owners.size()==1 && owners[0].Name==expected.Name && owners[0].Profile.Name==expected.Profile.Name);
   values.put('SC01', row.Id==scheduled && row.CronExpression==expression && row.CronJobDetail.Name=='Meadow owner schedule');
  } finally { System.abortJob(scheduled); }
  Id delayed = System.scheduleBatch(new MeadowScheduleOwnerBatch(), 'Meadow owner batch', 60);
  try {
   CronTrigger row = [SELECT Id, OwnerId, NextFireTime FROM CronTrigger WHERE Id=:delayed];
   values.put('SB01', row.OwnerId == current);
   values.put('SC03', row.Id==delayed && row.NextFireTime!=null);
  } finally { System.abortJob(delayed); }
  return values;
 }
}
