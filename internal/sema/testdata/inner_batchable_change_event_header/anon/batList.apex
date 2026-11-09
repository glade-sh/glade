public class Job implements Database.Batchable<String> {
  public List<String> start(Database.BatchableContext bc) { return new List<String>{'a'}; }
  public void execute(Database.BatchableContext bc, List<String> scope) {}
  public void finish(Database.BatchableContext bc) {}
}
System.debug('OBS|compiled');
