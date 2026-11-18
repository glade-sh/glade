public class BatchFuture implements Database.Batchable<Integer> {
 String token;
 public BatchFuture(String token) { this.token=token; }
 public Iterable<Integer> start(Database.BatchableContext context) { AsyncProbe.log('batch-start-'+token); return new List<Integer>{1}; }
 public void execute(Database.BatchableContext context,List<Integer> rows) { AsyncProbe.log('batch-execute-'+token+'-'+rows.size()); }
 public void finish(Database.BatchableContext context) {
  AsyncProbe.log('batch-finish-'+token);
  try { AsyncProbe.futureFromFinish(token); AsyncProbe.log('future-accepted-'+token); }
  catch(Exception e) { AsyncProbe.log('future-error-'+e.getTypeName()+'|'+e.getMessage()); }
 }
}
