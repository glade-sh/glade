public class RunnerBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1, 2, 3};
  }

  public void execute(Database.BatchableContext context, List<Integer> scope) {
    for (Integer value : scope) {
      insert new Widget__c(Name = 'Batch-' + value);
    }
  }

  public void finish(Database.BatchableContext context) {
    insert new Widget__c(Name = 'Batch-Finish');
  }
}
