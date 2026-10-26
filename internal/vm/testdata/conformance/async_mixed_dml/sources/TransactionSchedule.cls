public class TransactionSchedule implements Schedulable {
    Boolean setup; String label;
    public TransactionSchedule(Boolean setup,String label) { this.setup=setup; this.label=label; }
    public void execute(SchedulableContext context) { TransactionDml.work(setup,label); }
}
