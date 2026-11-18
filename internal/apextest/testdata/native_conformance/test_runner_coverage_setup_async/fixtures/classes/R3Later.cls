public class R3Later implements Queueable {
    public void execute(QueueableContext ctx) {
        insert new Account(Name = 'R3-LATER');
    }
}
