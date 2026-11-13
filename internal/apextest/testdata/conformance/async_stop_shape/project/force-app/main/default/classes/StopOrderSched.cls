public class StopOrderSched implements Schedulable {
    public void execute(SchedulableContext ctx) {
        StopOrderMk.mark('sc');
    }
}
