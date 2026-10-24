public virtual class InstanceBatchBase implements Database.Batchable<SObject> { public Iterable<SObject> start(Database.BatchableContext context) { insert new Account(Name='instanceBatchStart'); return new List<SObject>{new Account()}; }
public void execute(Database.BatchableContext context, List<SObject> scope) { insert new Account(Name='instanceBatchExecute'); }
public void finish(Database.BatchableContext context) { insert new Account(Name='instanceBatchFinish'); } }
