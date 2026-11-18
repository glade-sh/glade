public class StubBatch implements Database.Batchable<Integer> {
 public Iterable<Integer> start(Database.BatchableContext context) { return new List<Integer>{1}; }
 public void execute(Database.BatchableContext context,List<Integer> rows) {}
 public void finish(Database.BatchableContext context) {}
}
