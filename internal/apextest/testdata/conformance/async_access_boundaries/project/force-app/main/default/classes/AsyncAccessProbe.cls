public without sharing class AsyncAccessProbe {
 public class Envelope { public List<AccessLevel> levels; public Envelope(AccessLevel value) { levels=new List<AccessLevel>{value,value}; } }
 public class AccessJob implements Queueable {
  private AccessLevel direct; private Envelope nested; private Boolean systemMode;
  public AccessJob(AccessLevel value,Boolean useSystem) { direct=value; nested=new Envelope(value); systemMode=useSystem; }
  public void execute(QueueableContext context) {
   AccessLevel canonical=systemMode?AccessLevel.SYSTEM_MODE:AccessLevel.USER_MODE;
   Map<String,Object> result=new Map<String,Object>{'queueSame'=>direct==canonical,'queueEqual'=>direct.equals(canonical),'nestedSame'=>nested.levels[0]==canonical,'aliasSame'=>direct===nested.levels[0],'repeatedSame'=>nested.levels[0]===nested.levels[1]};
   insert new Account(Name='access-result',Description=JSON.serialize(result));
  }
 }
 public class StampJob implements Queueable {
  private Boolean chain; private Id recordId;
  public StampJob(Boolean nextJob,Id target) { chain=nextJob; recordId=target; }
  public void execute(QueueableContext context) {
   if(recordId!=null) { update new Account(Id=recordId,Name='stamp-terminal',Description=String.valueOf(context.getJobId())); return; }
   Account row=new Account(Name='stamp-root',Description=String.valueOf(context.getJobId()));insert row;
   if(chain) System.enqueueJob(new StampJob(false,row.Id));
  }
 }
 public static Map<String,Object> observe(String shape) {
  if(shape=='chainRejected'||shape=='unchained'||shape=='terminalUpdate') return stamp(shape);
  Boolean systemMode=shape=='accessSystem'; AccessLevel value=systemMode?AccessLevel.SYSTEM_MODE:AccessLevel.USER_MODE;
  if(shape=='accessCustom') {
   try { value=AccessLevel.USER_MODE.withPermissionSetId('0PS000000000001'); }
   catch(Exception issue) { return new Map<String,Object>{'customError'=>issue.getTypeName()}; }
  }
  AccessLevel copy=(AccessLevel)value.clone(); AccessLevel alias=value;
  Map<String,Object> result=new Map<String,Object>{'cloneSame'=>value==copy,'cloneEqual'=>value.equals(copy),'cloneIdentity'=>value===copy,'aliasSame'=>value===alias,'constantSame'=>value==(systemMode?AccessLevel.SYSTEM_MODE:AccessLevel.USER_MODE)};
  try { JSON.serialize(value);result.put('json','accepted'); } catch(Exception issue) {result.put('json',issue.getTypeName());}
  try { JSON.serialize(new Envelope(value));result.put('nestedJson','accepted'); } catch(Exception issue) {result.put('nestedJson',issue.getTypeName());}
  try { JSON.deserialize('{}',AccessLevel.class);result.put('deserialize','accepted'); } catch(Exception issue) {result.put('deserialize',issue.getTypeName());}
  if(Test.isRunningTest()) {
   Test.startTest();System.enqueueJob(new AccessJob(value,systemMode));Test.stopTest();
   Account row=[SELECT Description FROM Account WHERE Name='access-result' LIMIT 1];result.put('transport',JSON.deserializeUntyped(row.Description));
  }
  return result;
 }
 private static Map<String,Object> stamp(String shape) {
  if(!Test.isRunningTest()) return new Map<String,Object>{'boundary'=>'requiresTestDrain'};
  Id target;
  if(shape=='terminalUpdate') { Account row=new Account(Name='stamp-pending');insert row;target=row.Id; }
  Test.startTest();Id jobId=System.enqueueJob(new StampJob(shape=='chainRejected',target));String drainError;
  try { Test.stopTest(); } catch(Exception issue) { drainError=issue.getTypeName(); }
  List<Account> rows=[SELECT Name,Description FROM Account WHERE Name LIKE 'stamp-%'];
  return new Map<String,Object>{'drainError'=>drainError,'count'=>rows.size(),'terminal'=>rows.size()==1&&rows[0].Name=='stamp-terminal','rootStamp'=>rows.size()==1&&(Id)rows[0].Description==jobId};
 }
}
