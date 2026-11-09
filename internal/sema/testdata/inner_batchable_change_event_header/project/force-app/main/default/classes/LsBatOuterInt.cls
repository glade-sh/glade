public class LsBatOuterInt implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext bc) { return new List<Integer>{1}; }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {}
  public void finish(Database.BatchableContext bc) {}
}
