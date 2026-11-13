public class FixedSearchJob implements Queueable {
    Id t; String mode; String q; String extra;
    public FixedSearchJob(Id t0,String m,String q0,String e){t=t0;mode=m;q=q0;extra=e;}
    public void execute(QueueableContext c){ update new Account(Id=t,Description=FixedSearchHelper.work(mode,q,extra)); }
}
