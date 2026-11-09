public class LsBatInnerAcct {
 public class Job implements Database.Batchable<Account> {
  public List<Account> start(Database.BatchableContext bc) { return new List<Account>(); }
  public void execute(Database.BatchableContext bc, List<Account> scope) {}
  public void finish(Database.BatchableContext bc) {}
}
}
