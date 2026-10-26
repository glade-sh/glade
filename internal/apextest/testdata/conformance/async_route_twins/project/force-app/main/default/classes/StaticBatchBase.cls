public virtual class StaticBatchBase implements Database.Batchable<SObject> { public static Iterable<SObject> start(Database.BatchableContext context) { insert new Account(Name='staticBatchStart'); return new List<SObject>{new Account()}; }
public static void execute(Database.BatchableContext context, List<SObject> scope) { insert new Account(Name='staticBatchExecute'); }
public static void finish(Database.BatchableContext context) { insert new Account(Name='staticBatchFinish'); } }
