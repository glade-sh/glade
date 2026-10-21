public class TransactionStaticState {
 public static Integer seed=0;
 public static Integer hits=0;
 public static Boolean publishing=false;
 public static void save(String phase,Integer value) {insert new Account(Name=phase,NumberOfEmployees=value);}
 public static Integer read(String phase) {List<Account> rows=[SELECT NumberOfEmployees FROM Account WHERE Name=:phase];return rows.isEmpty()?-1:Integer.valueOf(rows[0].NumberOfEmployees);}
 public static void publishOne(Boolean afterCommit) {
  save('publisher seed',seed);
  publishing=true;
  Database.SaveResult result;
  if(afterCommit) result=EventBus.publish(new CommittedSignal__e());
  else result=EventBus.publish(new ImmediateSignal__e());
  save('publish success',result.isSuccess()?1:0);
  save('publisher saw trigger',read('trigger seed'));
  publishing=false;
  save('publisher after',1);
 }
 @future public static void publishFuture(Boolean afterCommit) {publishOne(afterCommit);}
 public static void receive() {
  save('trigger seed',seed);
  save('trigger publishing',publishing?1:0);
  save('trigger saw after',read('publisher after'));
  hits++;
 }
 public static Map<String,Integer> observe(String mode,Boolean afterCommit) {
  seed=7;
  if(mode=='batch') insert new Account(Name='batch seed');
  Test.startTest();
  if(mode=='direct') publishOne(afterCommit);
  if(mode=='queue') System.enqueueJob(new TransactionStaticQueue(afterCommit));
  if(mode=='future') publishFuture(afterCommit);
  if(mode=='batch') Database.executeBatch(new TransactionStaticBatch(afterCommit),1);
  Test.stopTest();
  Test.getEventBus().deliver();
  return new Map<String,Integer>{'publisherSeed'=>read('publisher seed'),'triggerSeed'=>read('trigger seed'),'triggerPublishing'=>read('trigger publishing'),'triggerSawAfter'=>read('trigger saw after'),'publisherSawTrigger'=>read('publisher saw trigger'),'publishSuccess'=>read('publish success'),'testHits'=>hits,'testSeed'=>seed};
 }
}
