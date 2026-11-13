public class FixedSearchBatch implements Database.Batchable<SObject> {
    Id t; String mode; String q; String extra;
    public FixedSearchBatch(Id t0,String m,String q0,String e){t=t0;mode=m;q=q0;extra=e;}
    public Database.QueryLocator start(Database.BatchableContext c){ return Database.getQueryLocator([SELECT Id FROM Account WHERE Id=:t]); }
    public void execute(Database.BatchableContext c,List<SObject> scope){ update new Account(Id=t,Description=FixedSearchHelper.work(mode,q,extra)); }
    public void finish(Database.BatchableContext c){}
}
