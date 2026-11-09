public class Job implements Database.Batchable<Object> {
  public Iterable<Object> start(Database.BatchableContext bc) { return new List<Object>{'a'}; }
  public void execute(Database.BatchableContext bc, List<Object> scope) {}
  public void finish(Database.BatchableContext bc) {}
}
System.debug('OBS|compiled');
